//
// PigYacc
// ------
// Copyright 2014 Philippe Hilger (PeerGum)
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.
//

package main

import (
	"bufio"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"
)

const (
	VERSION     = "0.1"
	BLANKSPACES = " \t"
)

const (
	STATE_INIT = iota
	STATE_FINISHED
	STATE_ERROR
	STATE_STACK
	STATE_SLASH
	STATE_STAR
	STATE_LINECOMMENT
	STATE_CCOMMENT
	STATE_PERCENT
	STATE_LEXRULES
	STATE_YACCRULES
	STATE_ACTION
	STATE_ACTIONBLOCK
	STATE_ACTIONEND

	USER_STATE
)

const (
	TOKEN_DOUBLESLASH = 256 + iota
	TOKEN_SLASHSTAR
	TOKEN_STARSLASH
	TOKEN_PERCENT
	TOKEN_COMMENTLINE
	TOKEN_INSTRUCTION
	TOKEN_EOF
	TOKEN_REGEXP
	TOKEN_BLOCKSTART
	TOKEN_ACTIONBLOCK
	TOKEN_ACTION
	TOKEN_BLOCKEND
	TOKEN_RETURN
	TOKEN_STATE
	TOKEN_TOKEN
	TOKEN_LEN
	TOKEN_VALUE
	TOKEN_ERROR

	USER_TOKEN
)

const (
	CMD_LEX = iota
	CMD_YACC
	CMD_INIT
	CMD_TOKEN
	CMD_STATE
)

var keywords = map[string]int{
	"return": TOKEN_RETURN,
	"state":  TOKEN_STATE,
	"token":  TOKEN_TOKEN,
	"len":    TOKEN_LEN,
	"value":  TOKEN_VALUE,
	"error":  TOKEN_ERROR,
}

type Token struct {
	id    int
	char  rune
	value interface{}
}

type State struct {
	current int
	token   *Token
}

type Yacc struct {
	source   *bufio.Reader
	states   []*State
	tokens   chan *Token
	position int
}

var (
	app      string  = filepath.Base(os.Args[0])
	fVersion *bool   = flag.Bool("v", false, "Show Version")
	fYacc    *string = flag.String("y", "grammar.pigy", "File with PigYacc rules")
	fName    *string = flag.String("f", "", "Source file to parse")
	fDebug   *bool   = flag.Bool("d", false, "Debug Mode")
	fOutput  *string = flag.String("o", "parser-defs.go", "Definition file to create")
	flags    []string
	args     []string
	tokens   = make([]string, 0, 50)
	states   = []string{"_INIT"}
)

func init() {
	flag.Parse()
	args = flag.Args()

	if *fVersion {
		showVersion()
	}
}

func main() {
	fmt.Printf("Welcome to %s.\n", app)

	if *fYacc == "" {
		fmt.Println("No PigYacc file.")
		os.Exit(1)
	}
	var yaccFile *os.File
	var err error

	if yaccFile, err = os.Open(*fYacc); err != nil {
		fmt.Printf("Can't open file %s", *fYacc)
		os.Exit(1)
	}
	defer yaccFile.Close()

	yacc := Yacc{
		source: bufio.NewReader(yaccFile),
		states: []*State{
			{
				current: STATE_INIT,
				token: &Token{
					id:    0,
					char:  0,
					value: "",
				},
			},
		},
		tokens: make(chan *Token),
	}

	done := make(chan int)
	go getTokens(yacc.tokens, done)

	for !yacc.finished() {
		if err := yacc.nextState(); err != nil {
			if err != io.EOF {
				fmt.Println("Error: ", err)
				os.Exit(1)
			}
			break
		}
	}
	done <- 0

}

func showVersion() {
	fmt.Printf("%s, version %s", app, VERSION)
	os.Exit(0)
}

func getTokens(tokens chan *Token, done chan int) {
	result := ""
	finished := false
	for !finished {
		select {
		case token := <-tokens:
			fmt.Printf("[%d: %s]\n", token.id, token.value)
			result += token.value.(string)
			//handleToken(token)
		case <-done:
			finished = true
		}
	}

}

//
// getNewState handles current state
//
func (yacc *Yacc) nextState() error {
	switch yacc.getState().current {
	case STATE_INIT:
		return yacc.stateInit()
	case STATE_FINISHED:
	case STATE_STACK:
	case STATE_ERROR:
		return yacc.stateError()
	case STATE_SLASH:
		return yacc.stateSlash()
	case STATE_CCOMMENT:
		return yacc.stateCComment()
	case STATE_STAR:
		return yacc.stateStar()
	case STATE_LINECOMMENT:
		return yacc.stateLineComment()
	case STATE_PERCENT:
		return yacc.statePercent()
	case STATE_YACCRULES:
		return yacc.stateyaccRules()
	case STATE_ACTION:
		return yacc.stateAction()
	case STATE_ACTIONBLOCK:
		return yacc.stateActionBlock()
	case STATE_ACTIONEND:
		return yacc.stateActionEnd()
	default:
		return yacc.stateError()
	}

	return nil
}

func (yacc *Yacc) getState() *State {
	currentState := len(yacc.states) - 1
	return yacc.states[currentState]
}

func (yacc *Yacc) pushState(state *State) error {
	yacc.states = append(yacc.states, state)
	//fmt.Printf("\nUp %d: %d\n", len(yacc.states), state.current)
	return nil
}

func (yacc *Yacc) replaceState(state *State) error {
	yacc.popState()
	yacc.pushState(state)
	return nil
}

func (yacc *Yacc) popState() {
	currentState := len(yacc.states) - 1
	yacc.states = yacc.states[:currentState]
	//fmt.Printf("\nDown %d: %d\n-- ", len(yacc.states), yacc.getState().current)
}

func (yacc *Yacc) getToken() *Token {
	return yacc.getState().token
}

func (yacc *Yacc) replaceToken(token *Token) {
	yacc.getState().token = token
}

func (yacc *Yacc) finished() bool {
	return (yacc.getState().current == STATE_FINISHED)
}

func (yacc *Yacc) setErrorState(error) {
	yacc.getState().current = STATE_ERROR
}

func (yacc *Yacc) printTokenValue() {
	fmt.Println("Token:", yacc.getState().token.value)
}

func (yacc *Yacc) getNext() (c rune, err error) {
	if c, _, err = yacc.source.ReadRune(); err != nil {
		yacc.tokens <- &Token{
			id:    TOKEN_EOF,
			char:  0,
			value: err.Error(),
		}
		return 0, err
	}
	if c != '\n' {
		yacc.position++
	}
	return
}

func (yacc *Yacc) checkKeyword() {
	value := yacc.getToken().value.(string)
	if len(value) > 0 {
		found := false
		for keyword, tokenId := range keywords {
			if keyword == value {
				token := &Token{
					id:    tokenId,
					char:  0,
					value: value,
				}
				yacc.tokens <- token
				logMsg("Token: ", token.value)
				yacc.getToken().value = ""
				found = true
				break
			}
		}
		if !found {
			for _, usertoken := range tokens {
				if usertoken == value {
					token := &Token{
						id:    USER_TOKEN,
						char:  0,
						value: value,
					}
					yacc.tokens <- token
					logMsg("User token: ", token.value)
					yacc.getToken().value = ""
					found = true
					break
				}
			}
		}
		if !found {
			for _, userstate := range states {
				if userstate == value {
					token := &Token{
						id:    USER_STATE,
						char:  0,
						value: value,
					}
					yacc.tokens <- token
					logMsg("User state: ", token.value)
					yacc.getToken().value = ""
					found = true
					break
				}
			}
		}
		if !found {
			token := &Token{
				id:    TOKEN_ERROR,
				char:  0,
				value: "ERR: " + value,
			}
			yacc.tokens <- token
			logMsg("Token: ", token.value)
		}
	}
	token := &Token{
		id:    0,
		char:  0,
		value: "",
	}
	yacc.replaceToken(token)
}

func (yacc *Yacc) checkCommand() {
	value := yacc.getToken().value.(string)
	token := &Token{
		id:    TOKEN_INSTRUCTION,
		char:  0,
		value: value,
	}
	yacc.tokens <- token
	logMsg("Token: ", token.value)
	fields := strings.Fields(value)
	switch fields[0] {
	case "yacc":
		token = &Token{
			id:    0,
			char:  0,
			value: "",
		}
		state := &State{
			current: STATE_YACCRULES,
			token:   token,
		}
		yacc.popState()
		yacc.replaceState(state)
		return
	case "only":
		logMsg("Only:", strings.Join(fields[1:], " "))
	case "except":
		logMsg("Except:", strings.Join(fields[1:], " "))
	case "include":
		logMsg("Include file:", strings.Join(fields[1:], ", "))
	case "output":
		logMsg("Output file (yaccer):", strings.Join(fields[1:], ", "))
	case "token":
		tokenList := strings.Replace(strings.Join(fields[1:], ","), " ", "", -1)
		for _, token := range strings.Split(tokenList, ",") {
			if token != "" {
				tokens = append(tokens, token)
			}
		}
		logMsg("Token(s):", strings.Join(tokens, ", "))
	case "state":
		stateList := strings.Replace(strings.Join(fields[1:], ","), " ", "", -1)
		for _, state := range strings.Split(stateList, ",") {
			if state != "" {
				states = append(states, state)
			}
		}
		logMsg("State(s):", strings.Join(states, ", "))
	case "alias":
		logMsg("Alias:", fields[1], "for", fields[2])
	}
	yacc.popState()
}

func (yacc *Yacc) checkComments(c rune) error {
	switch {
	case c == '/':
		token := &Token{
			id:    '/',
			char:  '/',
			value: c,
		}
		state := &State{
			current: STATE_SLASH,
			token:   token,
		}
		if yacc.pushState(state) != nil {
			return errors.New("Oops, can't push state!")
		}
	case c == '#':
		token := &Token{
			id:    '#',
			char:  '#',
			value: string(c),
		}
		state := &State{
			current: STATE_LINECOMMENT,
			token:   token,
		}
		if yacc.pushState(state) != nil {
			return errors.New("Oops, can't push state!")
		}
	}

	return nil
}

//
// Basic state
//
func (yacc *Yacc) stateInit() error {
	logMsg("=== INITIAL STATE ===")
	for yacc.getState().current == STATE_INIT {
		c, err := yacc.getNext()
		if err != nil {
			return err
		}
		yacc.checkComments(c)
		// check if we left init mode
		if yacc.getState().current != STATE_INIT {
			break
		}
		switch {
		case c == '%' && yacc.position == 0:
			token := &Token{
				id:    0,
				char:  0,
				value: "",
			}
			state := &State{
				current: STATE_PERCENT,
				token:   token,
			}
			if yacc.pushState(state) != nil {
				return errors.New("Oops, can't push state!")
			}
			//yacc.tokens <- token
		case c == '\r':
		case c == '\n':
			yacc.position = -1
		case strings.IndexRune(BLANKSPACES, c) >= 0:
			// skip blanks
		default:
			token := &Token{
				id:    0,
				char:  c,
				value: string(c),
			}
			yacc.replaceToken(token)
			yacc.tokens <- token
			logMsg("Token: ", token.value)
		}
	}
	return nil
}

//
// slash can be the beginning of a c-style comment or c++ comment line
//
func (yacc *Yacc) stateSlash() error {
	for yacc.getState().current == STATE_SLASH {
		c, err := yacc.getNext()
		if err != nil {
			return err
		}
		switch {
		case c == '/':
			// line comment
			token := &Token{
				id:    TOKEN_DOUBLESLASH,
				char:  0,
				value: "//",
			}
			state := &State{
				current: STATE_LINECOMMENT,
				token:   token,
			}
			yacc.replaceState(state)
			//yacc.tokens <- token
		case c == '*':
			// C style comment
			token := &Token{
				id:    TOKEN_SLASHSTAR,
				char:  0,
				value: "/*",
			}
			state := &State{
				current: STATE_CCOMMENT,
				token:   token,
			}
			yacc.replaceState(state)
			//yacc.tokens <- token
		case strings.IndexRune(BLANKSPACES, c) >= 0:
		default:
			yacc.tokens <- yacc.getToken()
			token := &Token{
				id:    0,
				char:  c,
				value: yacc.getToken().value.(string) + string(c),
			}
			yacc.popState()
			yacc.replaceToken(token)
			yacc.tokens <- token
			logMsg("Token: ", token.value)
		}
	}
	return nil
}

//
// C-style comment... expecting */ to leave
//
func (yacc *Yacc) stateCComment() error {
	logMsg("=== C COMMENT ===")
	for yacc.getState().current == STATE_CCOMMENT {
		c, err := yacc.getNext()
		if err != nil {
			return err
		}
		token := &Token{
			id:    0,
			char:  c,
			value: yacc.getToken().value.(string) + string(c),
		}
		switch c {
		case '*':
			state := &State{
				current: STATE_STAR,
				token:   token,
			}
			if yacc.pushState(state) != nil {
				return errors.New("Oops, can't push state!")
			}
		default:
			yacc.replaceToken(token)
			//yacc.tokens <- token
		}
	}
	return nil
}

//
// we're waiting for a slash to leave the comment
//
func (yacc *Yacc) stateStar() error {
	for yacc.getState().current == STATE_STAR {
		c, err := yacc.getNext()
		if err != nil {
			return err
		}
		switch c {
		case '/':
			// end of c-style comment
			token := &Token{
				id:    TOKEN_STARSLASH,
				char:  0,
				value: yacc.getToken().value.(string) + "/",
			}
			yacc.popState()
			yacc.popState()
			yacc.tokens <- token
			logMsg("Token: ", token.value)
		case '*':
			token := &Token{
				id:    '*',
				char:  '*',
				value: yacc.getToken().value.(string) + "*",
			}
			//yacc.tokens <- yacc.getToken()
			yacc.replaceToken(token)
		default:
			yacc.tokens <- yacc.getToken()
			token := &Token{
				id:    0,
				char:  c,
				value: yacc.getToken().value.(string) + string(c),
			}
			yacc.popState()
			yacc.replaceToken(token)
			yacc.tokens <- token
			logMsg("Token: ", token.value)

		}
	}
	return nil
}

//
// we're waiting for the end of line
//
func (yacc *Yacc) stateLineComment() error {
	logMsg("=== INLINE COMMENT ===")
	for yacc.getState().current == STATE_LINECOMMENT {
		c, err := yacc.getNext()
		if err != nil {
			return err
		}
		switch c {
		case '\r':
			// do nothing (CR)
		case '\n':
			yacc.position = -1
			token := &Token{
				id:    TOKEN_COMMENTLINE,
				char:  0,
				value: yacc.getState().token.value,
			}
			yacc.popState()
			yacc.tokens <- token
			//yacc.replaceToken(token)
			logMsg("Token: ", token.value)

			//yacc.printTokenValue()
		default:
			token := &Token{
				id:    0,
				char:  c,
				value: yacc.getToken().value.(string) + string(c),
			}
			yacc.replaceToken(token)
			//yacc.tokens <- token
		}
	}
	return nil
}

//
// instruction/command mode
//
func (yacc *Yacc) statePercent() error {
	logMsg("=== INSTRUCTION STATE ===")
	for yacc.getState().current == STATE_PERCENT {
		c, err := yacc.getNext()
		if err != nil {
			return err
		}
		yacc.checkComments(c)
		// check if we left init mode
		if yacc.getState().current != STATE_PERCENT {
			break
		}
		switch {
		case c == '\r':
		case c == '\n':
			yacc.position = -1
			yacc.checkCommand()
		default:
			token := &Token{
				id:    0,
				char:  c,
				value: yacc.getToken().value.(string) + string(c),
			}
			yacc.replaceToken(token)
			//yacc.tokens <- token
		}
	}
	return nil
}

//
// regular expression
//
func (yacc *Yacc) stateyaccRules() error {
	logMsg("=== yacc RULES STATE ===")
	for yacc.getState().current == STATE_YACCRULES {
		c, err := yacc.getNext()
		if err != nil {
			return err
		}
		yacc.checkComments(c)
		// check if we left init mode
		if yacc.getState().current != STATE_YACCRULES {
			break
		}
		switch {
		case c == '%' && yacc.position == 0:
			token := &Token{
				id:    0,
				char:  0,
				value: "",
			}
			state := &State{
				current: STATE_PERCENT,
				token:   token,
			}
			yacc.pushState(state)
		case (c == '\t' || c == '\n') && yacc.position > 0:
			token := &Token{
				id:    TOKEN_REGEXP,
				char:  c,
				value: yacc.getToken().value,
			}
			yacc.tokens <- token
			logMsg("Token: ", token.value)
			token = &Token{
				id:    0,
				char:  0,
				value: "",
			}
			state := &State{
				current: STATE_ACTION,
				token:   token,
			}
			yacc.replaceState(state)
		case c == '\r':
		case c == '\n':
			yacc.position = -1
			token := &Token{
				id:    0,
				char:  0,
				value: "",
			}
			yacc.replaceToken(token)
			//yacc.tokens <- token
		default:
			token := &Token{
				id:    0,
				char:  c,
				value: yacc.getToken().value.(string) + string(c),
			}
			yacc.replaceToken(token)
			//yacc.tokens <- token
		}
	}
	return nil
}

//
// action
//
func (yacc *Yacc) stateAction() error {
	logMsg("=== yacc ACTION STATE ===")
	for yacc.getState().current == STATE_ACTION {
		c, err := yacc.getNext()
		if err != nil {
			return err
		}
		yacc.checkComments(c)
		// check if we left init mode
		if yacc.getState().current != STATE_ACTION {
			break
		}
		switch {
		case strings.IndexRune(BLANKSPACES, c) >= 0:
			yacc.checkKeyword()
		case c == '\r':
		case c == '{' && yacc.position > 0:
			token := &Token{
				id:    TOKEN_BLOCKSTART,
				char:  c,
				value: "{",
			}
			yacc.tokens <- token
			logMsg("Token: ", token.value)
			token = &Token{
				id:    0,
				char:  0,
				value: "",
			}
			state := &State{
				current: STATE_ACTIONBLOCK,
				token:   token,
			}
			yacc.replaceState(state)
		case c == '\n' && yacc.getToken().value.(string) != "":
			yacc.position = -1
			yacc.checkKeyword()
			token := &Token{
				id:    0,
				char:  0,
				value: "",
			}
			state := &State{
				current: STATE_YACCRULES,
				token:   token,
			}
			yacc.replaceState(state)
		case c == '\n':
			yacc.position = -1
			token := &Token{
				id:    0,
				char:  0,
				value: "",
			}
			state := &State{
				current: STATE_YACCRULES,
				token:   token,
			}
			yacc.replaceState(state)
		default:
			token := &Token{
				id:    0,
				char:  c,
				value: yacc.getToken().value.(string) + string(c),
			}
			yacc.replaceToken(token)
			//yacc.tokens <- token
		}
	}
	return nil
}

//
// action block
//
func (yacc *Yacc) stateActionBlock() error {
	logMsg("=== yacc ACTION BLOCK STATE ===")
	for yacc.getState().current == STATE_ACTIONBLOCK {
		c, err := yacc.getNext()
		if err != nil {
			return err
		}
		yacc.checkComments(c)
		// check if we left init mode
		if yacc.getState().current != STATE_ACTIONBLOCK {
			break
		}
		switch {
		case strings.IndexRune(BLANKSPACES, c) >= 0 || c == '\n':
			yacc.checkKeyword()
			if c == '\n' {
				yacc.position = -1
			}
		case c == '\r':
		case c == '}':
			token := &Token{
				id:    TOKEN_BLOCKEND,
				char:  c,
				value: yacc.getToken().value.(string) + string("}"),
			}
			yacc.tokens <- token
			logMsg("Token: ", token.value)
			token = &Token{
				id:    0,
				char:  0,
				value: "",
			}
			state := &State{
				current: STATE_ACTIONEND,
				token:   token,
			}
			yacc.replaceState(state)
		default:
			token := &Token{
				id:    0,
				char:  c,
				value: yacc.getToken().value.(string) + string(c),
			}
			yacc.replaceToken(token)
			//yacc.tokens <- token
		}
	}
	return nil
}

func (yacc *Yacc) stateActionEnd() error {
	logMsg("=== yacc ACTION END STATE ===")
	for yacc.getState().current == STATE_ACTIONEND {
		c, err := yacc.getNext()
		if err != nil {
			return err
		}
		yacc.checkComments(c)
		// check if we left init mode
		if yacc.getState().current != STATE_ACTIONEND {
			break
		}
		switch {
		case strings.IndexRune(BLANKSPACES, c) >= 0:
		case c == '\r':
		case c == '\n':
			token := &Token{
				id:    0,
				char:  0,
				value: "",
			}
			state := &State{
				current: STATE_YACCRULES,
				token:   token,
			}
			yacc.replaceState(state)
		default:
		}
	}
	return nil
}

func (yacc *Yacc) stateError() error {
	return errors.New(yacc.getToken().value.(string))
}

func logMsg(v ...interface{}) {
	if *fDebug {
		log.Println(v)
	}
}
