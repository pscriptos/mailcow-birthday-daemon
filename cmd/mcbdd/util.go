package main

import (
	"crypto/rand"
	"fmt"
	"math/big"
	"strconv"
	"strings"
)

const (
	ConstPassgenChars = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ!@#$%^&*()_+-=[]{}\\|;':\",.<>/?`~0123456789"
)

func randomElement(s string) (string, error) {
	n, err := rand.Int(rand.Reader, big.NewInt(int64(len(s))))
	if err != nil {
		return "", fmt.Errorf("failed to generate random integer: %w", err)
	}
	return string(s[n.Int64()]), nil
}

func randomPassword(length int) (string, error) {
	pass := make([]byte, length)
	for i := range pass {
		char, err := randomElement(ConstPassgenChars)
		if err != nil {
			return "", err
		}
		pass[i] = []byte(char)[0]
	}
	return string(pass), nil
}

func sanitizeBirthday(input string) (uint16, uint16, uint16, error) {
	input = strings.ReplaceAll(input, "-", "")
	switch len(input) {
	case 4:
		mm, err := strconv.ParseUint(input[0:2], 10, 16)
		if err != nil {
			return 0, 0, 0, err
		}
		dd, err := strconv.ParseUint(input[2:4], 10, 16)
		if err != nil {
			return 0, 0, 0, err
		}
		return 0, uint16(mm), uint16(dd), nil
	case 8:
		yyyy, err := strconv.ParseUint(input[0:4], 10, 16)
		if err != nil {
			return 0, 0, 0, err
		}
		mm, err := strconv.ParseUint(input[4:6], 10, 16)
		if err != nil {
			return 0, 0, 0, err
		}
		dd, err := strconv.ParseUint(input[6:8], 10, 16)
		if err != nil {
			return 0, 0, 0, err
		}
		if yyyy == 1604 {
			yyyy = 0
		}
		return uint16(yyyy), uint16(mm), uint16(dd), nil
	}
	return 0, 0, 0, fmt.Errorf("birthday prop format unknown: %s", input)
}

// parseNotificationTrigger konvertiert eine Uhrzeit im Format "HH:MM" in eine
// iCal-Duration (z.B. "PT8H", "PT9H30M"), die als VALARM-Trigger relativ zum
// Start eines Ganztags-Events (Mitternacht) verwendet wird.
func parseNotificationTrigger(timeStr string) (string, error) {
	parts := strings.Split(timeStr, ":")
	if len(parts) != 2 {
		return "", fmt.Errorf("invalid time format: %s (expected HH:MM)", timeStr)
	}
	hours, err := strconv.Atoi(parts[0])
	if err != nil || hours < 0 || hours > 23 {
		return "", fmt.Errorf("invalid hours in time: %s", timeStr)
	}
	minutes, err := strconv.Atoi(parts[1])
	if err != nil || minutes < 0 || minutes > 59 {
		return "", fmt.Errorf("invalid minutes in time: %s", timeStr)
	}
	if hours == 0 && minutes == 0 {
		return "PT0S", nil
	}
	trigger := "PT"
	if hours > 0 {
		trigger += fmt.Sprintf("%dH", hours)
	}
	if minutes > 0 {
		trigger += fmt.Sprintf("%dM", minutes)
	}
	return trigger, nil
}
