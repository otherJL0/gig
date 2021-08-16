package main

import (
	"bufio"
	"database/sql"
	"embed"
	"flag"
	"fmt"
	"log"
	"os"
	"path"
	"strings"

	_ "modernc.org/sqlite"
)

//go:embed db/gig.db
var db embed.FS

var cwd string
var gitignore string

type LocalIgnore struct {
	filetype    string
	pattern     string
	isHidden    int
	isDirectory int
}

func NewLocalIgnore(fileLine string) *LocalIgnore {
	var hidden int
	if fileLine[0] == '.' {
		hidden = 1
	} else {
		hidden = 0
	}

	var directory int
	if fileLine[len(fileLine)-1] == '/' {
		directory = 1
	} else {
		directory = 0
	}
	return &LocalIgnore{
		filetype:    "LOCALIGNORE",
		pattern:     fileLine,
		isHidden:    hidden,
		isDirectory: directory,
	}
}

func (localIgnore *LocalIgnore) ToInsertStatement() string {
	return fmt.Sprintf("('%s','%s',%d,%d)", localIgnore.filetype, localIgnore.pattern, localIgnore.isHidden, localIgnore.isDirectory)
}

func status(args []string) {
	cmd := flag.NewFlagSet("status", flag.ExitOnError)
	cmd.Parse(args)

}

func add(args []string) {
	add := flag.NewFlagSet("add", flag.ExitOnError)
	add.Parse(args)

	if len(add.Args()) == 0 {
		fmt.Println("Please enter language to add")
		os.Exit(0)
	}
	languages := strings.ToUpper(strings.Join(add.Args(), "','"))

	db, err := sql.Open("sqlite", "db/gig.db")
	if err != nil {
		log.Fatal(err)
	}

	// Parse gitignore if present
	f, err := os.Open(gitignore)
	if err != nil {
		log.Fatal(err)
	} else {
		_, dbResetErr := db.Exec(`delete from gig
        where filetype = 'LOCALIGNORE';`)
		if dbResetErr != nil {
			log.Fatal(dbResetErr)
		}
		scanner := bufio.NewScanner(f)
		for scanner.Scan() {
			if scanner.Text() != "" && scanner.Text()[0] != '#' {
				element := NewLocalIgnore(scanner.Text())
				_, err := db.Exec(fmt.Sprintf(`insert into gig 
                (filetype, pattern, is_hidden, is_directory)
                values %s;`, element.ToInsertStatement()))
				if err != nil {
					log.Fatal(err)
				}
			}
		}
		f.Close()
	}

	query_string := fmt.Sprintf(`select distinct pattern 
            from gig 
            where FILETYPE in ('%s')
            and pattern not in (
                select pattern from gig where FILETYPE = 'LOCALIGNORE'
            )
            order by is_directory desc
            ,is_hidden desc
            ,pattern asc;`, languages)

	rows, err := db.Query(query_string)
	if err != nil {
		log.Fatal(err)
	}
	for rows.Next() {
		var pattern string
		if err = rows.Scan(&pattern); err != nil {
			log.Fatal(err)
		}
		fmt.Println(pattern)
		ignoreFile, err := os.OpenFile(gitignore, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
		if err != nil {
			log.Fatal(err)
		}
		defer ignoreFile.Close()

		_, writeErr := ignoreFile.WriteString(fmt.Sprintf("%s\n", pattern))
		if writeErr != nil {
			log.Fatal(writeErr)
		}
	}
}

func help() {
	fmt.Println("Printing help")
}

func init() {
	// Determine working directory
	wd, err := os.Getwd()
	if err != nil {
		log.Fatal(err)
	}
	cwd = wd
	gitignore = path.Join(cwd, ".gitignore")
	_, errExists := os.Stat(gitignore)
	if errExists != nil && os.IsNotExist(errExists) {
		gitignore = ""
	}

	// Check for directory contents
	var directoryContents = make(map[string]string)
	directoryContents["gitignore"] = path.Join(wd, ".gitignore")
	directoryContents["gitdirectory"] = path.Join(wd, ".git")
	for directoryElement, directoryPath := range directoryContents {
		_, err := os.Stat(directoryPath)
		if err == nil {
			fmt.Println(directoryElement + " is present")
		} else if os.IsNotExist(err) {
			fmt.Println(directoryElement + " is not present")
		}
	}

}

func main() {
	switch os.Args[1] {
	case "status":
		status(os.Args[2:])
	case "add":
		add(os.Args[2:])
	default:
		fmt.Println("Invalid command")
		help()
	}
}
