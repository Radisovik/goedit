package main

import (
	"fmt"
	"os"

	"golang.org/x/term"
)

func main() {
	// Get the terminal's file descriptor
	fd := int(os.Stdin.Fd())

	// Save the current terminal state
	oldState, err := term.MakeRaw(fd)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error setting raw mode: %v\n", err)
		return
	}
	defer term.Restore(fd, oldState) // Restore terminal when done

	// Print instructions
	fmt.Println("Press keys (arrow keys or regular keys). Press 'q' to quit. Ctrl+C to exit forcefully.")

	// Read keypresses from stdin
	buf := make([]byte, 3) // Buffer for reading keypresses
	for {
		n, err := os.Stdin.Read(buf)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error reading stdin: %v\n", err)
			break
		}

		// Process Arrow Keys (escape sequences)
		if n == 3 && buf[0] == 0x1b { // Escape Sequence (starts with 0x1b or '\033')
			switch buf[1] {
			case '[':
				switch buf[2] {
				case 'A':
					fmt.Println("Up arrow pressed")
				case 'B':
					fmt.Println("Down arrow pressed")
				case 'C':
					fmt.Println("Right arrow pressed")
				case 'D':
					fmt.Println("Left arrow pressed")
				default:
					fmt.Printf("Unknown escape sequence: %q\n", buf)
				}
			}
		}

		// Process Regular Keys (single-byte keys)
		if n == 1 {
			switch buf[0] {
			case 'q': // Quit on 'q'
				fmt.Println("Exiting...")
				return
			default: // Print other keys
				fmt.Printf("Key pressed: %q (ASCII: %d)\n", buf[0], buf[0])
			}
		}
	}
}
