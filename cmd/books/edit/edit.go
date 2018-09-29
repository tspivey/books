package edit

import (
	"errors"
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/tspivey/books"
)

var ErrUnknownCommand = errors.New("unknown command")

type DefaultCommand struct {
	Run    func(cmd *DefaultCommand, args string)
	Help   string
	parser *Parser
}

type Parser struct {
	book     *books.Book
	lib      *books.Library
	commands map[string]*DefaultCommand
}

func (p *Parser) RunCommand(cmd string, args string) error {
	dc, ok := p.commands[cmd]
	if !ok {
		return ErrUnknownCommand
	}
	dc.Run(dc, args)
	return nil
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
	parser := &Parser{book: book, lib: lib}
	c := func(cmd *DefaultCommand) *DefaultCommand {
		return &DefaultCommand{Run: cmd.Run, Help: cmd.Help, parser: parser}
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
	parser.commands = m
	return parser
}
