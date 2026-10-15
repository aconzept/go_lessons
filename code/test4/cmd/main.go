package main

import "test4"

func main() {
	list := test4.LinkedListNew()
	list.Push(1)
	list.Push(2)
	list.Push(3)

	print(list.Pop().Value)
	print(list.Pop().Value)
	print(list.Pop().Value)
	print(list.Pop() == nil)
	// print(list.Pop().Value)
	// print(list.Pop().Value)
}
