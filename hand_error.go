package main

import "fmt"

func showIfError(err error) {
	if err != nil {
		fmt.Println(err.Error())
	}
}

func panicIfError(err error) {
	if err != nil {
		panic(err)
	}
}
