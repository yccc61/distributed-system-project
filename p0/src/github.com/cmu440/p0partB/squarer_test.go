// Tests for SquarerImpl. Students should write their code in this file.

package p0partB

import (
	"fmt"
	"testing"
	"time"
)

const (
	timeoutMillis = 5000
)

func TestBasicCorrectness(t *testing.T) {
	fmt.Println("Running TestBasicCorrectness.")
	input := make(chan int)
	sq := SquarerImpl{}
	squares := sq.Initialize(input)
	go func() {
		input <- 2
	}()
	timeoutChan := time.After(time.Duration(timeoutMillis) * time.Millisecond)
	select {
	case <-timeoutChan:
		t.Error("Test timed out.")
	case result := <-squares:
		if result != 4 {
			t.Error("Error, got result", result, ", expected 4 (=2^2).")
		}
	}
}

func TestYourFirstGoTest(t *testing.T) {
	// Test if the squares close properly
	fmt.Println("Running the first go Test")
	input := make(chan int)
	sq := SquarerImpl{}
	squares := sq.Initialize(input)
	go func() {
		input <- 42
	}()
	sq.Close()
	timeoutChan := time.After(time.Duration(timeoutMillis) * time.Millisecond)
	//If sq properly closed, it means it is imposible to receive a message from the channel again
	//Therefore, we expected time out.
	select {
	case <-squares:
		t.Error("Expected Time Out")
	case <-timeoutChan:
		return
	}
}

func TestYourSecondGoTest(t *testing.T) {
	//Test data race does not exist when inserting continuous numbers into the channels.
	fmt.Println("Running the second go Test")
	input := make(chan int)
	sq := SquarerImpl{}
	squares := sq.Initialize(input)
	go func() {
		for i := 1; i < 100; i++ {
			input <- i
		}
	}()
	for j := 1; j < 100; j++ {
		if <-squares != j*j {
			t.Error("Wrong number")
		}
	}
	sq.Close()

}
