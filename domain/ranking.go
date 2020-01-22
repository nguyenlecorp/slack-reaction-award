package domain

type Nominate struct {
	Count   int
	Message SlackMessage
}

type Category struct {
	Omoro   []Nominate
	Variety []Nominate
	Amount  []Nominate
}

type Reaction struct {
	Key   string
	Value int
}

type Ranking struct {
	Category  *Category
	Reactions []Reaction
}
