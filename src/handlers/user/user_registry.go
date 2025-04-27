package user

import (
	"encoding/json"
	"fmt"
	"os"
	"sync"

	"github.com/rs/zerolog/log"
	"golang.org/x/crypto/bcrypt"
)

type UserRegistry struct {
	users map[string]User

	tokens     map[AccessToken]string
	tokensLock sync.RWMutex
}

func NewRegistry() *UserRegistry {
	return &UserRegistry{
		users:  make(map[string]User),
		tokens: make(map[AccessToken]string),
	}
}

func (u *UserRegistry) NewToken(name string) AccessToken {

	var token AccessToken
	token.New()

	u.tokensLock.Lock()
	defer u.tokensLock.Unlock()

	for {
		_, ok := u.tokens[token]

		if !ok {
			break
		}

		token.New()
	}

	u.tokens[token] = name

	return token
}

func (u *UserRegistry) CheckToken(tokenStr string) (string, bool) {

	var token AccessToken

	if err := token.FromString(tokenStr); err != nil {
		return "", false
	}

	u.tokensLock.RLock()
	defer u.tokensLock.RUnlock()

	name, ok := u.tokens[token]

	if ok {
		return name, true
	}
	return "", false
}

func (u *UserRegistry) HasUser(name string) (bool) {

	_, ok := u.users[name]

	return ok
}

func (u *UserRegistry) Login(name, password string) (string, bool) {

	user, ok := u.users[name]

	if !ok {
		return "", false
	}

	err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(password))

	if err != nil {

		if err != bcrypt.ErrMismatchedHashAndPassword {
			log.Warn().Err(err).Msg("")
		}

		return "", false
	}

	token := u.NewToken(name)

	return token.String(), true
}

func (u *UserRegistry) AddUser(username, password string) error {

	_, ok := u.users[username]

	if ok {
		return fmt.Errorf("user already exists")
	}

	pass, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)

	if err != nil {
		return fmt.Errorf("could not hash password")
	}

	u.users[username] = User{
		Username: username,
		Password: string(pass),
	}

	return nil
}

func (u *UserRegistry) LoadFromFile(path string) error {

	var users []User

	fileBytes, err := os.ReadFile(path)

	if err != nil {
		return err
	}

	if err := json.Unmarshal(fileBytes, &users); err != nil {
		return err
	}

	for _, user := range users {

		if user.Username == "" {
			log.Warn().Msg("skipping user with empty name")
			continue
		}
		if user.Password == "" {
			log.Warn().Str("user", user.Username).Msg("skipping user with empty password")
			continue
		}

		cost, err := bcrypt.Cost([]byte(user.Password))

		if err != nil {
			log.Warn().Str("user", user.Username).Msg("skipping user whos password is invalid")
			continue
		}

		log.Info().Str("user", user.Username).Int("cost", cost).Msg("registering user")

		u.users[user.Username] = user
	}

	return nil
}

func (u *UserRegistry) SaveToFile(path string) error {

	var users []User

	users = make([]User, 0, len(u.users))

	for _, user := range u.users {

		users = append(users, user)
	}

	data, err := json.MarshalIndent(users, "", "    ")

	if err != nil {
		return err
	}

	log.Info().Str("file", path).Msg("saving user registry")
	return os.WriteFile(path, data, 0600)
}
