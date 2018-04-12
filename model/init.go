package model

var globalClient Client

func InitGlobalClient(initFunc func() (Client, error)) error {
	c, err := initFunc()
	if err != nil {
		return err
	}

	globalClient = c
	return nil
}

func C() Client {
	return globalClient
}
