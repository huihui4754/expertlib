package types

type PossibleIntentions struct {
	IntentName        string  `json:"intent_name"`
	IntentDescription string  `json:"intent_description"`
	Probability       float64 `json:"probability"`
}
