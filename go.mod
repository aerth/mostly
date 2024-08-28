module github.com/aerth/mostly

go 1.22

toolchain go1.22.5

require go.etcd.io/bbolt v1.3.11

require (
	golang.org/x/sync v0.8.0 // indirect
	golang.org/x/sys v0.24.0 // indirect
)

retract v0.0.5 // unixtimestamp sql issue, fixed in v0.0.6