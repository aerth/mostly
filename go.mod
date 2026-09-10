module github.com/aerth/mostly

go 1.26.0

require go.etcd.io/bbolt v1.5.0

require (
	golang.org/x/sync v0.20.0 // indirect
	golang.org/x/sys v0.48.0 // indirect
)

retract v0.0.5 // unixtimestamp sql issue, fixed in v0.0.6
