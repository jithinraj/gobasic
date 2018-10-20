// Package eval contains our evaluator
//
// This is pretty simple:
//
//  * The program is an array of tokens.
//
//  * We have one statement per line.
//
//  * We handle the different types of statements in their own functions.
//

package eval

import (
	"fmt"
	"math/rand"
	"os"
	"strconv"

	"github.com/skx/gobasic/token"
	"github.com/skx/gobasic/tokenizer"
)

// Interpreter holds our state.
type Interpreter struct {

	// The program we execute is nothing more than an array of tokens.
	program []token.Token

	// We execute from the given offset.
	//
	// Sequential exection just means bumping this up by one each
	// time we execute an instruction, or pick off the arguments to
	// one.
	//
	// But set it to 17, or some other random value, and you've got
	// a GOTO implemented.
	offset int

	// A stack for handling GOSUB/RETURN calls
	gstack *Stack

	// vars holds the variables set in the program, via LET.
	vars *Variables
}

// New is our constructor.
//
// Given a lexer we store all the tokens it produced in our array, and
// initialise some other state.
func New(stream *tokenizer.Tokenizer) *Interpreter {
	t := &Interpreter{offset: 0}

	// setup a stack for holding line-numbers for GOSUB/RETURN
	t.gstack = NewStack()

	// setup storage for variable-contents
	t.vars = NewVars()

	// save the tokens one by one, until we hit the end.
	for {
		tok := stream.NextToken()
		if tok.Type == token.EOF {
			break
		}
		t.program = append(t.program, tok)
	}

	return t
}

////
//
// Helpers for stuff
//
////

// factor
func (e *Interpreter) factor() int {

	tok := e.program[e.offset]
	switch tok.Type {
	case token.LBRACKET:
		// skip past the lbracket
		e.offset++

		// handle the expr
		ret := e.expr()

		// skip past the rbracket
		tok = e.program[e.offset]
		if tok.Type != token.RBRACKET {
			fmt.Printf("Unclosed bracket around expression!\n")
			os.Exit(1)
		}
		e.offset++

		// Return the result of the sub-expression
		return (ret)
	case token.INT:
		i, err := strconv.Atoi(tok.Literal)
		if err == nil {
			e.offset++
			return i
		}
		fmt.Printf("Failed to convert %s -> int %s\n", tok.Literal, err.Error())
		os.Exit(3)

	case token.IDENT:

		//
		// This is a kinda place-holder for handling
		// literals that are function-calls.
		//
		// We don't really have any at the moment, but I've
		// implemented "RND()" and "ABS(XX)/ABS(NN)" as a
		// quick proof of concept.
		//
		// TODO:
		//  Rethink this whole approach.  It is misguided at
		// best, and broken at worst.
		//
		if tok.Literal == "RND" {
			// skip past RND
			e.offset++

			// skip past (
			e.offset++

			// skip past the )
			e.offset++

			// 0-99, inclusive.
			return rand.Intn(100)
		}
		if tok.Literal == "ABS" {

			// skip past ABS
			e.offset++

			// skip past (
			e.offset++

			// get the variable
			val := e.program[e.offset]
			e.offset++

			// ObReminder: We could just use expr() ..

			// skip past the )
			e.offset++

			// Run ABS
			if val.Type == token.INT {
				num, _ := strconv.Atoi(val.Literal)
				if num < 0 {
					return -1 * num
				}
				return num
			}
			if val.Type == token.IDENT {
				n := e.vars.Get(val.Literal)
				nVal, ok := n.(int)
				if ok {
					if nVal < 0 {
						return -1 * nVal
					}
					return nVal
				}
			}
		}
		// Get the contents of the variable.
		val := e.vars.Get(tok.Literal)

		iVal, ok := val.(int)
		if ok {
			e.offset++
			return iVal
		}
		fmt.Printf("GET(%s) wasn't an int!\n", tok.Literal)
		os.Exit(3)
	}

	fmt.Printf("factor() - unhandled token: %v\n", tok)
	os.Exit(33)
	return -1
}

// terminal
func (e *Interpreter) term() int {

	f1 := e.factor()

	tok := e.program[e.offset]

	for tok.Type == token.ASTERISK ||
		tok.Type == token.SLASH ||
		tok.Type == token.MOD {

		// skip the operator
		e.offset++

		// get the next factor
		f2 := e.factor()

		if tok.Type == token.ASTERISK {
			f1 = f1 * f2
		}
		if tok.Type == token.SLASH {
			f1 = f1 / f2
		}
		if tok.Type == token.MOD {
			f1 = f1 % f2
		}

		// repeat?
		tok = e.program[e.offset]
	}
	return f1
}

// expression
func (e *Interpreter) expr() int {

	t1 := e.term()

	tok := e.program[e.offset]

	for tok.Type == token.PLUS ||
		tok.Type == token.MINUS {

		// skip the operator
		e.offset++

		t2 := e.term()

		if tok.Type == token.PLUS {
			t1 = t1 + t2
		}
		if tok.Type == token.MINUS {
			t1 = t1 - t2
		}

		// repeat?
		tok = e.program[e.offset]
	}

	return t1
}

// runForLoop handles a FOR loop
func (e *Interpreter) runForLoop() error {
	// we expect "ID = NUM to NUM [STEP NUM]"

	// Bump past the FOR token
	e.offset++

	// We now expect a token
	target := e.program[e.offset]
	e.offset++
	if target.Type != token.IDENT {
		return fmt.Errorf("Expected IDENT after FOR, got %v\n", target)
	}

	// Now an EQUALS
	eq := e.program[e.offset]
	e.offset++
	if eq.Type != token.ASSIGN {
		return fmt.Errorf("Expected = after 'FOR %s' , got %v\n", target.Literal, eq)
	}

	// Now an integer
	startI := e.program[e.offset]
	e.offset++
	if startI.Type != token.INT {
		return fmt.Errorf("Expected INT after 'FOR %s=', got %v\n", target.Literal, startI)
	}

	start, err := strconv.Atoi(startI.Literal)
	if err != nil {
		return fmt.Errorf("Failed to convert %s to an int %s\n", startI.Literal, err.Error())
	}

	// Now TO
	to := e.program[e.offset]
	e.offset++
	if to.Type != token.TO {
		return fmt.Errorf("Expected TO after 'FOR %s=%s', got %v\n", target.Literal, startI, to)
	}

	// Now an integer
	endI := e.program[e.offset]
	e.offset++
	if endI.Type != token.INT {
		return fmt.Errorf("Expected INT after 'FOR %s=%s TO', got %v\n", target.Literal, startI, endI)
	}

	end, err := strconv.Atoi(endI.Literal)
	if err != nil {
		return fmt.Errorf("Failed to convert %s to an int %s\n", endI.Literal, err.Error())
	}

	// Default step is 1.
	stepI := "1"

	// Is the next token a step?
	if e.program[e.offset].Type == token.STEP {
		e.offset++

		s := e.program[e.offset]
		e.offset++
		if s.Type != token.INT {
			return fmt.Errorf("Expected INT after 'FOR %s=%s TO %s STEP', got %v\n", target.Literal, startI, endI, s)
		}
		stepI = s.Literal
	}

	step, err := strconv.Atoi(stepI)
	if err != nil {
		fmt.Errorf("Failed to convert %s to an int %s\n", stepI, err.Error())
	}

	//
	// Now we can record the important details of the for-loop
	// in a hash.
	//
	// The key observersions here are that all the magic
	// really involved in the FOR-loop happens at the point
	// you interpret the "NEXT X" section.
	//
	// Handling the NEXT statement involves:
	//
	//  Incrementing the step-variable
	//  Looking for termination
	//  If not over then "jumping back".
	//
	// So for a for-loop we just record the start/end conditions
	// and the address of the body of the loop - ie. the next
	// token - so that the next-handler can GOTO there.
	//
	// It is almost beautifully elegent.
	//
	f := ForLoop{id: target.Literal,
		offset: e.offset,
		start:  start,
		end:    end,
		step:   step}

	//
	// Set the variable to the starting-value
	//
	e.vars.Set(target.Literal, start)

	//
	// And record our loop - keyed on the name of the variable
	// which is used as the index.  This allows easy and natural
	// nested-loops.
	//
	// Did I say this is elegent?
	//
	AddForLoop(f)

	return nil
}

////
//
// Statement-handlers
//
////

// runGOSUB handles a control-flow change
func (e *Interpreter) runGOSUB() error {

	// Skip the GOSUB-instruction itself
	e.offset++

	// Get the target
	target := e.program[e.offset]

	// We expect the next token to be an int
	// If we had variables ..
	if target.Type != token.INT {
		return (fmt.Errorf("ERROR: GOSUB should be followed by an integer\n"))
	}

	//
	// We want to store the return address on our GOSUB-stack,
	// so that the next RETURN will continue execution at the
	// next instruction.
	//
	// Because we only support one statement per-line we can
	// handle that by bumping forward.  That should put us on the
	// LINENO of the following-line.
	//
	e.offset++
	e.gstack.Push(e.offset)

	//
	// Scan the whole program from the beginning.
	//
	// TODO: Build a lookup-table at load-time.
	//
	for i := 0; i < len(e.program); i++ {

		// Did we find a line-number?
		if e.program[i].Type == token.LINENO {

			// Does it match?
			if e.program[i].Literal == target.Literal {
				e.offset = i
				return nil
			}
		}
	}

	return (fmt.Errorf("Failed to GOSUB %s\n", target.Literal))
}

// runGOTO handles a control-flow change
func (e *Interpreter) runGOTO() error {

	// Skip the GOTO-instruction
	e.offset++

	// Get the GOTO-target
	target := e.program[e.offset]

	// We expect the next token to be an int
	if target.Type != token.INT {
		return fmt.Errorf("ERROR: GOTO should be followed by an integer\n")
	}

	//
	// Scan the whole program from the beginning.
	//
	// TODO: Build a lookup-table at load-time.
	//
	for i := 0; i < len(e.program); i++ {

		// Did we find a line-number?
		if e.program[i].Type == token.LINENO {

			// Does it match the target we're aiming for?
			if e.program[i].Literal == target.Literal {
				e.offset = i
				return nil
			}
		}
	}

	return fmt.Errorf("Failed to GOTO %s\n", target.Literal)
}

// runIF handles conditional testing.
// runLET handles variable creation/updating.
func (e *Interpreter) runLET() error {

	// Bump past the LET token
	e.offset++

	// We now expect an ID
	target := e.program[e.offset]
	e.offset++
	if target.Type != token.IDENT {
		return fmt.Errorf("Expected IDENT after LET, got %v\n", target)
	}

	// Now "="
	assign := e.program[e.offset]
	if assign.Type != token.ASSIGN {
		return fmt.Errorf("Expected assignment after LET x, got %v\n", assign)
	}
	e.offset++

	// now we're at the expression/value/whatever
	res := e.expr()

	e.vars.Set(target.Literal, res)
	return nil
}

// runNEXT handles the NEXT statement
func (e *Interpreter) runNEXT() error {
	// Bump past the NEXT token
	e.offset++

	// Get the identifier
	target := e.program[e.offset]
	e.offset++
	if target.Type != token.IDENT {
		return fmt.Errorf("Expected IDENT after NEXT, got %v\n", target)
	}

	// OK we've found the tail of a loop
	//
	// We need to bump the value of the given variable by the offset
	// and compare it against the max.
	//
	// If the max hasn't finished we loop around again.
	//
	// If it has we remove the for-loop
	//
	data := GetForLoop(target.Literal)

	//
	// Get the variable value, and increase it.
	//
	cur := e.vars.Get(target.Literal)
	iVal, _ := cur.(int)
	iVal += data.step

	//
	// Set it
	//
	e.vars.Set(target.Literal, iVal)

	//
	// Have we finnished?
	//
	if data.finished {
		RemoveForLoop(target.Literal)
		return nil
	}

	//
	// If we've reached our limit we mark this as complete,
	// but note that we dont' terminate to allow the actual
	// end-number to be inclusive.
	//
	if iVal == data.end {
		data.finished = true

		// updates-in-place.  bad name
		AddForLoop(data)
	}

	//
	// Otherwise loop again
	//
	e.offset = data.offset
	return nil
}

// runPRINT handles a print!
func (e *Interpreter) runPRINT() error {

	// Bump past the PRINT token
	e.offset++

	// Now keep lookin for things to print until we hit a newline.
	for e.offset < len(e.program) {

		// Get the token
		tok := e.program[e.offset]

		// End of the line?
		if tok.Type == token.NEWLINE {
			return nil
		}

		// We expect to handle "int", "string", and ",".
		if tok.Type == token.INT || tok.Type == token.STRING {
			fmt.Printf("%s", tok.Literal)
		} else if tok.Type == token.COMMA {
			fmt.Printf(" ")
		} else if tok.Type == token.IDENT {

			// Get the contents of the variable.
			val := e.vars.Get(tok.Literal)

			// TODO: Type - we just look for "string", then "int".
			sVal, ok := val.(string)
			if ok {
				fmt.Printf("%s", sVal)
			} else {
				iVal, ok := val.(int)
				if ok {
					fmt.Printf("%d", iVal)
				}
			}
		} else {
			// OK we're not printing:
			//
			//  an int
			//  a string
			//  a variable
			//
			// As a fall-back we'll assume we've been given
			// an expression, and print the result.
			//
			out := e.expr()
			fmt.Printf("%d\n", out)
		}
		e.offset++
	}

	return nil
}

// REM handles a REM statement
func (e *Interpreter) runREM() error {

	// Skip over all content until we hit the end
	// of the program, or a newline.
	//
	// Whichever comes first.
	//
	for e.offset < len(e.program) {
		tok := e.program[e.offset]
		if tok.Type == token.NEWLINE {
			return nil
		}
		e.offset++
	}

	return nil
}

// RETURN handles a control-flow operation
func (e *Interpreter) runRETURN() error {

	// Stack can't be empty
	if e.gstack.Empty() {
		return fmt.Errorf("RETURN without GOSUB\n")
	}

	// Get the return address
	ret, err := e.gstack.Pop()
	if err != nil {
		return fmt.Errorf("Error handling RETURN: %s\n", err.Error())
	}

	// Return execution where we left off.
	e.offset = ret
	return nil
}

////
//
// Our core public API
//
////

// Run launches our program!
func (e *Interpreter) Run() error {

	//
	// We walk our series of tokens.
	//
	for e.offset < len(e.program) {

		//
		// Get the current token
		//
		tok := e.program[e.offset]

		var err error

		//
		// Handle this token
		//
		switch tok.Type {
		case token.NEWLINE:
			// NOP
		case token.LINENO:
			// NOP
		case token.END:
			return nil
		case token.FOR:
			err = e.runForLoop()
		case token.GOSUB:
			err = e.runGOSUB()
		case token.GOTO:
			err = e.runGOTO()
		case token.LET:
			err = e.runLET()
		case token.NEXT:
			err = e.runNEXT()
		case token.PRINT:
			err = e.runPRINT()
		case token.REM:
			err = e.runREM()
		case token.RETURN:
			err = e.runRETURN()
		default:
			err = fmt.Errorf("Token not handled: %v\n", tok)
		}

		if err != nil {
			return err
		}
		//
		// Handle the next statement.
		//
		e.offset++
	}

	return nil
}

// GetVariable returns the contents of the given variable.
// Useful for testing/embedding.
func (e *Interpreter) GetVariable(id string) int {
	n := e.vars.Get(id)
	nVal, ok := n.(int)
	if ok {
		return nVal
	}
	fmt.Printf("Failed to cast result of GetVariable(%s) to int!\n",
		id)
	os.Exit(1)

	// FAKE
	return 1
}