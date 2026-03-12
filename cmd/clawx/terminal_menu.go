package main

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"syscall"
	"unsafe"
)

func promptMenu(title string, options []menuOption) (string, error) {
	if len(options) == 0 {
		return "", nil
	}

	// Use raw TTY menu by default (openclaw-like behavior). You can force
	// fallback line mode with CLAWX_TUI=false when terminal compatibility
	// is problematic.
	if interactiveInputAvailable() && os.Getenv("CLAWX_TUI") != "false" {
		if selected, ok, err := promptMenuTTY(title, options); err != nil {
			return "", err
		} else if ok {
			return selected, nil
		}
	}

	return promptMenuFallback(title, options)
}

func promptMenuTTY(title string, options []menuOption) (string, bool, error) {
	fd := int(os.Stdin.Fd())
	previous, err := makeRaw(fd)
	if err != nil {
		return "", false, nil
	}
	defer func() {
		_ = restoreTerminal(fd, previous)
		fmt.Fprintln(os.Stdout)
	}()

	selected := defaultMenuIndex(options)
	renderedLines := 0

	for {
		renderedLines = renderTTYMenu(title, options, selected, renderedLines)

		key, err := readMenuKey()
		if err != nil {
			return "", true, err
		}
		switch key.kind {
		case keyConfirm:
			return options[selected].key, true, nil
		case keyInterrupt:
			return "", true, fmt.Errorf("interrupted")
		case keyUp:
			selected = previousIndex(selected, len(options))
		case keyDown:
			selected = nextIndex(selected, len(options))
		case keyDigit:
			if key.index >= 0 && key.index < len(options) {
				return options[key.index].key, true, nil
			}
		default:
		}
	}
}

type menuKeyKind int

const (
	keyUnknown menuKeyKind = iota
	keyConfirm
	keyInterrupt
	keyUp
	keyDown
	keyDigit
)

type menuKey struct {
	kind  menuKeyKind
	index int
}

func readMenuKey() (menuKey, error) {
	readOne := func() (byte, error) {
		var b [1]byte
		n, err := os.Stdin.Read(b[:])
		if err != nil {
			return 0, err
		}
		if n == 0 {
			return 0, nil
		}
		return b[0], nil
	}

	ch, err := readOne()
	if err != nil {
		return menuKey{}, err
	}

	switch ch {
	case 0:
		return menuKey{kind: keyUnknown}, nil
	case '\r', '\n', ' ':
		return menuKey{kind: keyConfirm}, nil
	case 3:
		return menuKey{kind: keyInterrupt}, nil
	case 'k', 'K':
		return menuKey{kind: keyUp}, nil
	case 'j', 'J':
		return menuKey{kind: keyDown}, nil
	case 0x1b:
		second, err := readOne()
		if err != nil {
			return menuKey{}, err
		}
		third, err := readOne()
		if err != nil {
			return menuKey{}, err
		}
		if second == '[' && third == 'A' {
			return menuKey{kind: keyUp}, nil
		}
		if second == '[' && third == 'B' {
			return menuKey{kind: keyDown}, nil
		}
		return menuKey{kind: keyUnknown}, nil
	default:
		if ch >= '1' && ch <= '9' {
			return menuKey{kind: keyDigit, index: int(ch - '1')}, nil
		}
		return menuKey{kind: keyUnknown}, nil
	}
}

func promptMenuFallback(title string, options []menuOption) (string, error) {
	fmt.Fprintln(os.Stdout, title)
	for index, option := range options {
		fmt.Fprintf(os.Stdout, "  %d) %s\n", index+1, option.label)
	}
	fmt.Fprintln(os.Stdout, "输入编号后回车确认")
	defaultKey := options[defaultMenuIndex(options)].key
	reader := bufio.NewReader(os.Stdin)

	for {
		fmt.Fprint(os.Stdout, "选择编号并回车: ")
		value, err := readLooseLine(reader)
		if err != nil {
			return "", err
		}
		normalized := normalizeChoice(value)
		if normalized == "" {
			return defaultKey, nil
		}
		if digit := firstChoiceDigit(normalized); digit >= 0 && digit < len(options) {
			return options[digit].key, nil
		}
		for _, option := range options {
			for _, alias := range option.aliases {
				if normalized == normalizeChoice(alias) {
					return option.key, nil
				}
			}
		}
		fmt.Fprintf(os.Stdout, "无效选项: %s\n", value)
	}
}

func readLooseLine(reader *bufio.Reader) (string, error) {
	var line strings.Builder
	pendingCaret := false
	for {
		ch, err := reader.ReadByte()
		if err != nil {
			if errors.Is(err, io.EOF) {
				if pendingCaret {
					line.WriteByte('^')
				}
				return line.String(), nil
			}
			if line.Len() > 0 {
				if pendingCaret {
					line.WriteByte('^')
				}
				return line.String(), nil
			}
			return "", err
		}
		if pendingCaret {
			if ch == 'm' || ch == 'M' || ch == 'j' || ch == 'J' {
				return line.String(), nil
			}
			line.WriteByte('^')
			pendingCaret = false
		}
		if ch == '^' {
			pendingCaret = true
			continue
		}
		if ch == '\n' || ch == '\r' || ch < 0x20 || ch == 0x7f {
			return line.String(), nil
		}
		line.WriteByte(ch)
	}
}

func renderTTYMenu(title string, options []menuOption, selected int, renderedLines int) int {
	totalLines := len(options) + 2
	if renderedLines > 0 {
		fmt.Fprintf(os.Stdout, "\x1b[%dA", renderedLines)
	}

	writeMenuLine(title)
	for index, option := range options {
		prefix := "  "
		if index == selected {
			prefix = "> "
		}
		writeMenuLine(fmt.Sprintf("%s%d) %s", prefix, index+1, option.label))
	}
	writeMenuLine("上下键选择，回车确认，数字直达，Ctrl+C 取消")
	return totalLines
}

func writeMenuLine(line string) {
	fmt.Fprintf(os.Stdout, "\x1b[2K\r%s\n", line)
}

func defaultMenuIndex(options []menuOption) int {
	for index, option := range options {
		if option.selected {
			return index
		}
	}
	return 0
}

func previousIndex(current int, length int) int {
	if current <= 0 {
		return length - 1
	}
	return current - 1
}

func nextIndex(current int, length int) int {
	if current >= length-1 {
		return 0
	}
	return current + 1
}

func normalizeChoice(value string) string {
	normalized := strings.ToLower(strings.TrimSpace(value))
	normalized = strings.ReplaceAll(normalized, "^m", "")
	normalized = strings.ReplaceAll(normalized, "\r", "")
	return strings.TrimSpace(normalized)
}

func firstChoiceDigit(value string) int {
	for _, ch := range value {
		if ch >= '1' && ch <= '9' {
			return int(ch - '1')
		}
	}
	return -1
}

func makeRaw(fd int) (*syscall.Termios, error) {
	state, err := readTermios(fd)
	if err != nil {
		return nil, err
	}

	raw := *state
	raw.Lflag &^= syscall.ICANON | syscall.ECHO
	raw.Iflag &^= syscall.IXON | syscall.ICRNL
	raw.Cc[syscall.VMIN] = 1
	raw.Cc[syscall.VTIME] = 0

	if err := setTermios(fd, &raw); err != nil {
		return nil, err
	}

	return state, nil
}

func restoreTerminal(fd int, state *syscall.Termios) error {
	if state == nil {
		return nil
	}
	return setTermios(fd, state)
}

func ensureCookedStdin() (func(), error) {
	if !interactiveInputAvailable() {
		return func() {}, nil
	}
	fd := int(os.Stdin.Fd())
	state, err := readTermios(fd)
	if err != nil {
		return nil, err
	}

	cooked := *state
	cooked.Lflag |= syscall.ICANON | syscall.ECHO | syscall.ISIG
	cooked.Iflag |= syscall.ICRNL
	if err := setTermios(fd, &cooked); err != nil {
		return nil, err
	}
	return func() { _ = restoreTerminal(fd, state) }, nil
}

func readTermios(fd int) (*syscall.Termios, error) {
	state := &syscall.Termios{}
	if _, _, errno := syscall.Syscall6(
		syscall.SYS_IOCTL,
		uintptr(fd),
		uintptr(syscall.TCGETS),
		uintptr(unsafe.Pointer(state)),
		0,
		0,
		0,
	); errno != 0 {
		return nil, errno
	}
	return state, nil
}

func setTermios(fd int, state *syscall.Termios) error {
	if _, _, errno := syscall.Syscall6(
		syscall.SYS_IOCTL,
		uintptr(fd),
		uintptr(syscall.TCSETS),
		uintptr(unsafe.Pointer(state)),
		0,
		0,
		0,
	); errno != 0 {
		return errno
	}
	return nil
}
