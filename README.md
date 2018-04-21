# userdata

userdata will print updated coreos userdata with updates off

```sh
go get -u github.com/goller/userdata
userdata -region us-east-1  -instance i-123456
```

If the userdata does not need to be updated, the program will return.
If it does need to be updated, you'll see a diff of the changes and
will be prompted to type "y" if the changes seem ok.

Once you accept the changes the userdata will be printed to stdout.


## Notes
If you see changes in etcd2 unit like dashes becoming underscores, don't worry. That's fine.
