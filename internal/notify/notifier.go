package notify

type Notifier interface {
	Send(title, body string) error
}

type AndroidViewBinder interface {
	SetAndroidView(view uintptr)
}

func New() Notifier {
	return newNotifier()
}
