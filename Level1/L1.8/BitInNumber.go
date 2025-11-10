package main

import (
	"fmt"
)

func main() {
	var x int64
	var i int
	var bit int

	fmt.Print("Введите число: ")
	fmt.Scan(&x)

	fmt.Print("Введите номер бита: ")
	fmt.Scan(&i)

	fmt.Print("Введите значение (0 или 1): ")
	fmt.Scan(&bit)

	if bit == 1 {
		x = x | (1 << i)
	} else {
		x = x &^ (1 << i)
	}

	fmt.Println("Результат:", x)
}
