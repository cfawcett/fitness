package models

import "fitness/platform/database"

type Auth0Profile struct {
	Sub           string `json:"sub"`
	Name          string `json:"name"`
	GivenName     string `json:"given_name"`
	FamilyName    string `json:"family_name"`
	Nickname      string `json:"nickname"`
	Picture       string `json:"picture"`
	Email         string `json:"email"`
	EmailVerified bool   `json:"email_verified"`
}

type GroupedSets struct {
}

type ActivityHistory struct {
	Activity database.Activity
	Sets     []*database.GymSet
}
