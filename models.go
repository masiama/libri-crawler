package main

import "libri/ent"

type localBook struct {
	ent.Book
	imageURL string
}
