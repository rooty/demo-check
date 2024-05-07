package main_test

import (
	"math"
	"os"
	"testing"
)

func Exmp1(t *testing.T){
	got := math.Abs(-1)
	if got != 1 {
		t.Errorf("Abs(-1) = %f; want 1", got)
	}
}

func TestMain(m *testing.M) {
	// call flag.Parse() here if TestMain uses flags
	os.Exit(m.Run())
}
func TestConvert(t *testing.T){
	t.Log("Hello testing")
	//t.Fail()
}