package logger

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"strings"
	"sync"
	"time"

	waLog "go.mau.fi/whatsmeow/util/log"
)

// Plataformas como o Railway classificam tudo que sai em stderr como ERROR.
// O pacote github.com/gomessguii/logger usa o log padrão do Go (que escreve em
// stderr), então até mensagens [INFO] apareciam como erro. Este arquivo roteia
// cada linha para o stream correto de acordo com o nível: ERROR vai para
// stderr, o resto para stdout. Com LOG_TYPE=json as linhas saem como JSON
// estruturado ({"level","message",...}), formato que o Railway interpreta.

var (
	consoleMu   sync.Mutex
	consoleJSON bool
	stdout      io.Writer = os.Stdout
	stderr      io.Writer = os.Stderr
)

// Prefixos emitidos por github.com/gomessguii/logger.
var gomessguiiPrefixes = []struct {
	prefix string
	level  string
}{
	{"\033[41m[ERR]\033[0m ", "error"},
	{"\033[43m[WARN]\033[0m ", "warn"},
	{"\033[40m\033[37m[DEBUG]\033[0m ", "debug"},
	{"\033[44m[INFO]\033[0m ", "info"},
}

// SetupConsole redireciona o log padrão do Go para o roteador por nível.
// jsonFormat=true emite cada linha como JSON estruturado.
func SetupConsole(jsonFormat bool) {
	consoleMu.Lock()
	consoleJSON = jsonFormat
	consoleMu.Unlock()

	if jsonFormat {
		log.SetFlags(0)
	} else {
		log.SetFlags(log.LstdFlags)
	}
	log.SetOutput(stdLogWriter{})
}

// stdLogWriter recebe cada chamada do log padrão (uma por Write) e descobre o
// nível pelo prefixo do gomessguii/logger. Linhas sem prefixo são tratadas como info.
type stdLogWriter struct{}

func (stdLogWriter) Write(p []byte) (int, error) {
	line := strings.TrimRight(string(p), "\n")
	level := "info"
	for _, pf := range gomessguiiPrefixes {
		if idx := strings.Index(line, pf.prefix); idx >= 0 {
			level = pf.level
			if isJSON() {
				// Remove o prefixo colorido; o nível vai no campo "level".
				line = line[:idx] + line[idx+len(pf.prefix):]
			}
			break
		}
	}
	emit(level, "", line)
	return len(p), nil
}

func isJSON() bool {
	consoleMu.Lock()
	defer consoleMu.Unlock()
	return consoleJSON
}

type consoleEntry struct {
	Time    string `json:"time"`
	Level   string `json:"level"`
	Module  string `json:"module,omitempty"`
	Message string `json:"message"`
}

// emit escreve uma linha já formatada (modo texto) ou a serializa em JSON.
func emit(level, module, msg string) {
	consoleMu.Lock()
	defer consoleMu.Unlock()

	out := stdout
	if level == "error" {
		out = stderr
	}

	if consoleJSON {
		b, err := json.Marshal(consoleEntry{
			Time:    time.Now().Format(time.RFC3339Nano),
			Level:   level,
			Module:  module,
			Message: msg,
		})
		if err != nil {
			return
		}
		out.Write(append(b, '\n'))
		return
	}

	out.Write([]byte(msg + "\n"))
}

// waLogColors replica as cores do waLog.Stdout.
var waLogColors = map[string]string{
	"INFO":  "\033[36m",
	"WARN":  "\033[33m",
	"ERROR": "\033[31m",
}

var waLogLevels = map[string]int{
	"":      -1,
	"DEBUG": 0,
	"INFO":  1,
	"WARN":  2,
	"ERROR": 3,
}

// waConsoleLogger substitui o waLog.Stdout do whatsmeow (que escreve tudo,
// inclusive erros, em stdout) usando o mesmo roteamento por nível.
type waConsoleLogger struct {
	mod string
	min int
}

// NewWALogger cria um logger compatível com o whatsmeow. minLevel segue a
// semântica do waLog.Stdout: DEBUG, INFO, WARN, ERROR ou vazio (tudo).
func NewWALogger(module, minLevel string) waLog.Logger {
	return &waConsoleLogger{mod: module, min: waLogLevels[strings.ToUpper(minLevel)]}
}

func (w *waConsoleLogger) outputf(level, msg string, args ...any) {
	if waLogLevels[level] < w.min {
		return
	}
	text := fmt.Sprintf(msg, args...)
	if isJSON() {
		emit(strings.ToLower(level), w.mod, text)
		return
	}
	colorStart, colorReset := waLogColors[level], ""
	if colorStart != "" {
		colorReset = "\033[0m"
	}
	emit(strings.ToLower(level), w.mod, fmt.Sprintf("%s%s [%s %s] %s%s",
		time.Now().Format("15:04:05.000"), colorStart, w.mod, level, text, colorReset))
}

func (w *waConsoleLogger) Errorf(msg string, args ...any) { w.outputf("ERROR", msg, args...) }
func (w *waConsoleLogger) Warnf(msg string, args ...any)  { w.outputf("WARN", msg, args...) }
func (w *waConsoleLogger) Infof(msg string, args ...any)  { w.outputf("INFO", msg, args...) }
func (w *waConsoleLogger) Debugf(msg string, args ...any) { w.outputf("DEBUG", msg, args...) }
func (w *waConsoleLogger) Sub(mod string) waLog.Logger {
	return &waConsoleLogger{mod: fmt.Sprintf("%s/%s", w.mod, mod), min: w.min}
}
