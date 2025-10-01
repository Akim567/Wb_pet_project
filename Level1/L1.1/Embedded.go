package main

import (
	"fmt"
)

// Human — "родительская" структура с полями и методами
type Human struct {
	Name string
	Age  int
}

// Info — метод с value-receiver: читает состояние, ничего не меняет
func (h Human) Info() string {
	return fmt.Sprintf("%s, %d y/o", h.Name, h.Age)
}

// Birthday — метод с pointer-receiver: изменяет состояние Human
func (h *Human) Birthday() {
	h.Age++
}

// Action — "потомок" через композицию
type Action struct {
	*Human
}

// Свой собственный метод Action
func (a Action) Do(what string) {
	fmt.Printf("%s делает: %s\n", a.Name, what) // поля Human "поднялись"
}

// Конструктор, чтобы не забыть инициализировать вложенный *Human
func NewAction(name string, age int) Action {
	return Action{
		Human: &Human{
			Name: name,
			Age:  age,
		},
	}
}

func main() {
	// Создаём Action; внутри уже есть инициализированный *Human.
	a := NewAction("Akim", 22)

	// 1) Доступ к "поднятым" методам Human
	fmt.Println(a.Info()) // Human.Info()

	a.Birthday() // Human.Birthday()
	fmt.Println(a.Info())

	// 2) Доступ к "поднятым" полям Human напрямую
	fmt.Println("Имя:", a.Name, "Возраст:", a.Age)

	// 3) Свой метод Action
	a.Do("Метод класса Action")

	// 4) Явное обращение к родителю
	fmt.Println("Явный вызов родителя:", a.Human.Info())
}
