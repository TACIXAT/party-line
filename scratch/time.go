package main

import (
	"fmt"
	"time"
)

type Test struct {
	Time time.Time
}

func main() {
	fmt.Println(time.Now().Format("15:04:05"))

	// compares properly between timezones, guess I didn't have to .UTC() everything
	t0 := new(time.Time)
	err := t0.UnmarshalJSON([]byte("\"2018-08-02T01:09:57.504532053-04:00\""))
	fmt.Println(t0, time.Now())
	fmt.Println(time.Now().Sub(*t0))

	// confirm
	t1 := new(time.Time)
	err = t1.UnmarshalJSON([]byte("\"2018-08-02T09:12:48.397204721-07:00\""))
	if err != nil {
		panic(err)
	}

	t2 := new(time.Time)
	err = t2.UnmarshalJSON([]byte("\"2018-08-02T12:12:48.397204721-04:00\""))
	if err != nil {
		panic(err)
	}

	fmt.Println(t1, t2)
	fmt.Println(t1.Equal(*t2))

	fmt.Println("== IsZero ==")
	test := new(Test)
	fmt.Println(test.Time.IsZero())
	test.Time = time.Now().UTC()
	fmt.Println(test.Time.IsZero())
}
