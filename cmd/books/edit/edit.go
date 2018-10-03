// Package edit contains sub commands for editing books from the command line.
package edit

import (
	"errors"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"

	"github.com/tspivey/books"
)

// ErrUnknownCommand is returned when a command cannot be found by the given command name.
var ErrUnknownCommand = errors.New("unknown command")

// DefaultCommand contains fields used by all other edit commands.
type DefaultCommand struct {
	Run       func(cmd *DefaultCommand, args string)
	RunE      func(cmd *DefaultCommand, args string) error
	Help      string
	parser    *Parser
	completer func(cmd *DefaultCommand, s string) []string
}

// Parser contains the set of available commands, and the shared state for those commands.
type Parser struct {
	book     *books.Book
	lib      *books.Library
	commands map[string]*DefaultCommand
}

// RunCommand runs a command with the given arguments, returning ErrUnknownCommand if not found.
func (p *Parser) RunCommand(cmd string, args string) error {
	dc, ok := p.commands[cmd]
	if !ok {
		return ErrUnknownCommand
	}
	if dc.RunE != nil {
		return dc.RunE(dc, args)
	}
	dc.Run(dc, args)
	return nil
}

// Completer tries to complete a command and its arguments.
func (p *Parser) Completer(s string) []string {
	s = strings.TrimSpace(s)
	if s == "" {
		return []string{}
	}
	commands := []string{}
	for k := range p.commands {
		commands = append(commands, k)
	}
	sort.Strings(commands)
	for _, c := range commands {
		if p.commands[c].completer == nil {
			continue
		}
		l := p.commands[c].completer(p.commands[c], s)
		if len(l) > 0 {
			return l
		}
	}
	return []string{}
}

var authorsCmd = &DefaultCommand{
	Help: "Sets the authors of the currently edited book",
	Run: func(cmd *DefaultCommand, args string) {
		if args == "" {
			fmt.Fprintf(os.Stderr, "Usage: authors <authors>\n")
			return
		}
		newAuthors := strings.Split(args, " & ")
		cmd.parser.book.Authors = newAuthors
	},
	completer: func(cmd *DefaultCommand, s string) []string {
		if !strings.HasPrefix("authors", s) {
			return []string{}
		}
		return []string{"authors " + strings.Join(cmd.parser.book.Authors, " & ")}
	},
}

var titleCmd = &DefaultCommand{
	Help: "Sets the title of the currently edited book",
	Run: func(cmd *DefaultCommand, args string) {
		if args == "" {
			fmt.Fprintf(os.Stderr, "Usage: title <title>\n")
			return
		}
		cmd.parser.book.Title = args
	},
	completer: func(cmd *DefaultCommand, s string) []string {
		if !strings.HasPrefix("title", s) {
			return []string{}
		}
		return []string{"title " + cmd.parser.book.Title}
	},
}

var seriesCmd = &DefaultCommand{
	Help: "Sets the series of the currently edited book",
	Run: func(cmd *DefaultCommand, args string) {
		if args == "" {
			fmt.Fprintf(os.Stderr, "Usage: series <series>\n")
			return
		}
		cmd.parser.book.Series = args
	},
	completer: func(cmd *DefaultCommand, s string) []string {
		if !strings.HasPrefix("series", s) {
			return []string{}
		}
		return []string{"series " + cmd.parser.book.Series}
	},
}

var saveCmd = &DefaultCommand{
	Help: "Saves the currently edited book",
	Run: func(cmd *DefaultCommand, args string) {
		err := cmd.parser.lib.UpdateBook(*cmd.parser.book, true)
		if bee, ok := err.(books.BookExistsError); ok {
			if args == "-m" {
				err := cmd.parser.lib.MergeBooks([]int64{bee.BookID, cmd.parser.book.ID})
				if err != nil {
					fmt.Fprintf(os.Stderr, "Error merging books: %v\n", err)
					return
				}
				fmt.Printf("Merged into %d\n", bee.BookID)
			} else {
				fmt.Printf("A duplicate book already exists, id: %d. To merge, type save -m.\n", bee.BookID)
				return
			}
		} else if err != nil {
			fmt.Fprintf(os.Stderr, "error while updating book: %v\n", err)
			return
		}
	},
}

var showCmd = &DefaultCommand{
	Help: "Shows available commands",
	Run: func(cmd *DefaultCommand, args string) {
		fmt.Println("Title: ", cmd.parser.book.Title)
		fmt.Println("Authors: ", strings.Join(cmd.parser.book.Authors, " & "))
		fmt.Println("Series: ", cmd.parser.book.Series)
	},
}

var helpCmd = &DefaultCommand{
	Help: "Gets help",
}

var quitCmd = &DefaultCommand{
	Help: "quits the editor without saving",
	RunE: func(cmd *DefaultCommand, args string) error {
		return io.EOF
	},
}

func cmdHelp(commandsMap map[string]*DefaultCommand, cmd *DefaultCommand, args string) {
	commands := []string{}
	for k := range commandsMap {
		commands = append(commands, k)
	}
	sort.Strings(commands)
	for _, c := range commands {
		fmt.Println(c + " " + commandsMap[c].Help)
	}
}

// NewParser creates a new parser.
func NewParser(book *books.Book, lib *books.Library) *Parser {
	parser := &Parser{
		book: book,
		lib:  lib,
	}

	// Return a copy of a DefaultCommand  with a parser and completer added.
	c := func(cmd *DefaultCommand) *DefaultCommand {
		return &DefaultCommand{
			Run:       cmd.Run,
			RunE:      cmd.RunE,
			Help:      cmd.Help,
			parser:    parser,
			completer: cmd.completer,
		}
	}
	m := make(map[string]*DefaultCommand)
	m["authors"] = c(authorsCmd)
	m["title"] = c(titleCmd)
	m["series"] = c(seriesCmd)
	m["save"] = c(saveCmd)
	m["show"] = c(showCmd)
	m["help"] = c(helpCmd)
	m["help"].Run = func(cmd *DefaultCommand, args string) {
		cmdHelp(m, cmd, args)
	}
	m["quit"] = c(quitCmd)
	parser.commands = m
	return parser
}
