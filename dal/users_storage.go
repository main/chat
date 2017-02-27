package dal

import (
	"errors"
	"log"
	"time"

	"gopkg.in/mgo.v2"
	"gopkg.in/mgo.v2/bson"
)

const (
	defaultSessionLifetime = 31 * 24 * time.Hour
)

var (
	ErrNotFound = errors.New("Not found")
)

type UserDefinition struct {
	ID           bson.ObjectId `bson:"_id,omitempty"`
	Name         string
	Email        string
	RegistryDate time.Time
	PasswordHash string
	Confirmed    bool
	ConfirmId    string
}

type User struct {
	name    string
	session mgo.Session
}

func (u *User) Exists() (bool, error) {
	collection := *u.session.DB("chat").C("users")
	count, err := collection.Find(bson.M{"name": u.name}).Count()
	if err != nil {
		panic(err)
	}
	return count > 0
}

func (u *User) Delete() error {
	val, err := redis.Bool(u.session.Do("del", "user:"+u.login))
	if err != nil {
		return err
	}
	if !val {
		return ErrNotFound
	}
	return nil
}

func (u *User) GetPassword() (string, error) {
	pass, err := redis.String(u.session.Do("hget", "user:"+u.login, "pass"))
	if err != nil {
		return "", err
	}
	return pass, nil
}

func (u *User) SetPassword(pass string) error {
	_, err := u.session.Do("hset", "user:"+u.login, "pass", pass)
	if err != nil {
		return err
	}
	return nil
}

func (u *User) GetName() (string, error) {
	name, err := redis.String(u.session.Do("hget", "user:"+u.login, "name"))
	if err != nil {
		return "", err
	}
	return name, nil
}

func (u *User) SetName(name string) error {
	_, err := u.session.Do("hset", "user:"+u.login, "name", name)
	if err != nil {
		return err
	}
	return nil
}

func (u *User) CreateSession(id string, ex time.Duration) (*Session, error) {
	//	xid := xid.New().String()
	if ex == 0 {
		ex = defaultSessionLifetime
	}
	_, err := u.session.Do("multi")
	if err != nil {
		return nil, err
	}
	_, err = u.session.Do("hmset", "sess:"+id, "user", u.login)
	if err != nil {
		return nil, err
	}
	_, err = u.session.Do("expire", ex/time.Second)
	if err != nil {
		return nil, err
	}
	_, err = u.session.Do("exec")
	if err != nil {
		return nil, err
	}
	session := &Session{
		id:      "sess:" + id,
		session: u.session,
	}
	return session, nil
}

type Session struct {
	id      string
	session mgo.Session
}

func (s *Session) ProlongSession(ex time.Duration) error {
	if ex == 0 {
		ex = defaultSessionLifetime
	}
	ok, err := redis.Bool(s.session.Do("expire", "sess:"+s.id, ex/time.Second))
	if err != nil {
		return err
	}
	if !ok {
		return errors.New("redis cannot set expiration on session" + s.id)
	}
	return nil
}

func (s *Session) Delete() error {
	val, err := redis.Bool(s.session.Do("del", "sess:"+s.id))
	if err != nil {
		return err
	}
	if !val {
		return ErrNotFound
	}
	return nil
}

func (s *Session) PutString(key, value string) error {
	_, err := s.session.Do("hset", "sess:"+s.id, key, value)
	if err != nil {
		return err
	}
	return nil
}

func (s *Session) GetString(key string) (string, error) {
	val, err := redis.String(s.session.Do("hget", "sess:"+s.id, key))
	if err != nil {
		return "", err
	}
	return val, nil
}

func (s *Session) GetUser() (*User, error) {
	userId, err := redis.String(s.session.Do("hget", "sess:"+s.id, "user"))
	if err != nil {
		return nil, err
	}
	if userId == "" {
		return nil, ErrNotFound
	}
	user := &User{
		login:   userId,
		session: s.session,
	}

	return user, nil
}

type UsersStorage struct {
	session mgo.Session
}

func NewUsersStorage() (*UsersStorage, error) {
	session, err := mgo.Dial("127.0.0.1")

	defer session.Close()
	if err != nil {
		log.Println(err)
		return nil, err
	}
	return &UsersStorage{session: session}, nil

}

func (u *UsersStorage) CreateUser(login, name, pass string) (*User, error) {
	_, err := u.session.Do("hmset", "user:"+login, "name", name, "pass", pass)
	if err != nil {
		return nil, err
	}
	user := &User{
		login:   login,
		session: u.session,
	}
	return user, nil
}

func (u *UsersStorage) FindSessionById(id string) (*Session, error) {
	exists, err := redis.Bool(u.session.Do("exists", "sess:"+id))
	if err != nil {
		return nil, err
	}
	if !exists {
		return nil, ErrNotFound
	}
	session := &Session{
		id:      id,
		session: u.session,
	}
	return session, nil
}
