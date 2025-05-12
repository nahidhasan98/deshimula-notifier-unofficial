package interfacer

type Service interface {
	FetchAndProcessStories() error
}
