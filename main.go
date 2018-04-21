package main

import (
	"bufio"
	"encoding/base64"
	"flag"
	"fmt"
	"os"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/aws/external"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/coreos/coreos-cloudinit/config"
	"github.com/sergi/go-diff/diffmatchpatch"
)

var region = flag.String("region", "us-east-1", "aws region")
var instance = flag.String("instance", "", "aws instance to generate new user-data")

func main() {
	flag.Parse()
	if *instance == "" {
		flag.Usage()
		os.Exit(1)
	}
	cfg, err := external.LoadDefaultAWSConfig()
	if err != nil {
		panic(err)
	}

	cfg.Region = *region
	svc := ec2.New(cfg)
	input := &ec2.DescribeInstanceAttributeInput{
		Attribute:  "userData",
		InstanceId: aws.String(*instance),
	}

	req := svc.DescribeInstanceAttributeRequest(input)
	res, err := req.Send()
	if err != nil {
		panic(err)
	}

	if res.UserData == nil || res.UserData.Value == nil {
		panic("no userdata found")
	}
	data, err := base64.StdEncoding.DecodeString(*res.UserData.Value)
	if err != nil {
		panic(err)
	}

	conf, err := config.NewCloudConfig(string(data))
	if err != nil {
		panic(err)
	}
	updateIdx := -1
	lockIdx := -1
	for i := range conf.CoreOS.Units {
		switch conf.CoreOS.Units[i].Name {
		case "update-engine.service":
			if conf.CoreOS.Units[i].Command != "stop" {
				conf.CoreOS.Units[i].Command = "stop"
			} else {
				updateIdx = i
			}
		case "locksmithd.service":
			if conf.CoreOS.Units[i].Command != "stop" {
				conf.CoreOS.Units[i].Command = "stop"
			} else {
				lockIdx = i
			}
		}
	}

	if updateIdx >= 0 && lockIdx >= 0 {
		os.Exit(1)
	}

	if updateIdx == -1 {
		conf.CoreOS.Units = append(conf.CoreOS.Units, config.Unit{
			Name:    "update-engine.service",
			Command: "stop",
		})
	}

	if lockIdx == -1 {
		conf.CoreOS.Units = append(conf.CoreOS.Units, config.Unit{
			Name:    "locksmithd.service",
			Command: "stop",
		})
	}

	dmp := diffmatchpatch.New()
	diffs := dmp.DiffMain(string(data), conf.String(), false)
	fmt.Println(dmp.DiffPrettyText(diffs))

	reader := bufio.NewReader(os.Stdin)
	fmt.Println("if diff looks right, type y (user-data will follow)")
	text, _ := reader.ReadString('\n')
	if text == "y\n" {
		fmt.Printf("New user data:\n\n\n\n\n\n\n%s", conf.String())
	}
}
