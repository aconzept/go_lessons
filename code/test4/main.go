package test4

// create linked list
// iterate (next)
// push (append at end)
// pop (delete last and return)

// shift(append at 0)
// search

type linkedList struct {
	current *linkedListNode
	first   *linkedListNode
}

type linkedListNode struct {
	Value int
	next  *linkedListNode
}

func LinkedListNew() linkedList {
	return linkedList{}
}

func (r *linkedList) Reset() {
	r.current = r.first
}

func (r *linkedList) Next() bool {
	if r.current == nil || r.current.next == nil {
		return false
	}
	r.current = r.current.next
	return true
}

func (r *linkedList) Push(val int) {
	node := linkedListNode{}
	node.Value = val

	for r.Next() {
	}

	if r.current == nil {
		r.current = &node
		r.first = r.current
	} else {
		r.current.next = &node
	}
}

func (r *linkedList) Pop() *linkedListNode {
	if r.current == nil {
		return nil
	}
	r.Reset()
	for r.current != nil &&
		r.current.next != nil &&
		r.current.next.next != nil {
		r.Next()
	}
	var res *linkedListNode
	if r.current.next == nil {
		res = r.current
		r.first = nil
		r.current = nil
	} else {
		res = r.current.next
		r.current.next = nil
	}

	return res
}
