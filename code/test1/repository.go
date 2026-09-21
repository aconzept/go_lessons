package main

type User struct {
	Id   string
	Name string
}

type Order struct {
	Price int
}

type Page[T any] struct {
	List   []T
	Limit  int
	Offset int
}

type Repo[T any] struct {
	List []T
}

type IRepo[T any] interface {
	findAll(l int, o int) Page[T]
	findByName(pred func(arg T) bool) *T
}

var _ IRepo[any] = &Repo[any]{}

func (r *Repo[T]) findAll(l int, o int) Page[T] {
	return Page[T]{List: r.List, Limit: l, Offset: o}
}

func (r *Repo[T]) findByName(pred func(arg T) bool) *T {
	for _, value := range r.List {
		if pred(value) {
			return &value
		}
	}
	return nil
}

func NewUserRepo() *Repo[User] {
	return &Repo[User]{List: []User{{Id: "one", Name: "Vazgen"}, {Id: "two", Name: "Suren"}}}
}

func NewOrderRepo() *Repo[Order] {
	return &Repo[Order]{List: []Order{{Price: 10}, {Price: 27}}}
}