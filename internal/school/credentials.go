package school

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/joho/godotenv"
)

var (
	usernameEnvKeys = []string{
		"PASSPORT_UNDERGRADUATE_USERNAME",
	}
	graduateUsernameEnvKeys = []string{
		"PASSPORT_GRADUATE_USERNAME",
	}
	passwordEnvKeys = []string{
		"PASSPORT_PASSWORD",
	}
	totpEnvKeys = []string{
		"PASSPORT_TOTP",
	}
)

func ResolveCredentials(username, password, totp string) (Credentials, error) {
	return ResolveCredentialsForProgram(ProgramUndergraduate, username, password, totp)
}

func ResolveCredentialsForProgram(program Program, username, password, totp string) (Credentials, error) {
	envFile := readDotEnv()
	selectedUsernameKeys := usernameEnvKeys
	if program.IsGraduate() {
		selectedUsernameKeys = graduateUsernameEnvKeys
	}
	creds := Credentials{
		Username: firstNonEmpty(strings.TrimSpace(username), firstEnvOrFile(envFile, selectedUsernameKeys...)),
		Password: firstNonEmpty(password, firstEnvOrFile(envFile, passwordEnvKeys...)),
		TOTP:     firstNonEmpty(strings.TrimSpace(totp), firstEnvOrFile(envFile, totpEnvKeys...)),
	}

	var missing []string
	if creds.Username == "" {
		missing = append(missing, "username")
	}
	if creds.Password == "" {
		missing = append(missing, "password")
	}
	if creds.TOTP == "" {
		missing = append(missing, "totp")
	}
	if len(missing) > 0 {
		return Credentials{}, fmt.Errorf(
			"missing school credentials: %s (flags or env: %s / %s / %s)",
			strings.Join(missing, ", "),
			strings.Join(selectedUsernameKeys, ", "),
			strings.Join(passwordEnvKeys, ", "),
			strings.Join(totpEnvKeys, ", "),
		)
	}

	return creds, nil
}

func DetectCredentialPrograms() []Program {
	envFile := readDotEnv()
	var programs []Program
	if firstEnvOrFile(envFile, "PASSPORT_UNDERGRADUATE_USERNAME") != "" {
		programs = append(programs, ProgramUndergraduate)
	}
	if firstEnvOrFile(envFile, "PASSPORT_GRADUATE_USERNAME") != "" {
		programs = append(programs, ProgramGraduate)
	}
	return programs
}

func firstEnv(keys ...string) string {
	for _, key := range keys {
		if value := strings.TrimSpace(os.Getenv(key)); value != "" {
			return value
		}
	}
	return ""
}

func firstEnvOrFile(fileEnv map[string]string, keys ...string) string {
	if value := firstEnv(keys...); value != "" {
		return value
	}
	for _, key := range keys {
		if value := strings.TrimSpace(fileEnv[key]); value != "" {
			return value
		}
	}
	return ""
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func readDotEnv() map[string]string {
	wd, err := os.Getwd()
	if err != nil {
		return nil
	}

	for dir := wd; dir != filepath.Dir(dir); dir = filepath.Dir(dir) {
		path := filepath.Join(dir, ".env")
		values, err := godotenv.Read(path)
		if err == nil {
			return values
		}
	}
	return nil
}
