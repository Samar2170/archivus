package types

type DirEntry struct {
	ID        string
	Name      string
	IsDir     bool
	Extension string
	SignedUrl string
	Size      float64
	Path      string
	Thumbnail string

	NavigationPath string
}
