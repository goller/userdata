package main

import (
	"bufio"
	"encoding/base64"
	"flag"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/aws/external"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go/aws/endpoints"
	"github.com/coreos/coreos-cloudinit/config"
	"github.com/sergi/go-diff/diffmatchpatch"
)

var (
	all      = flag.Bool("all", false, "for each region find all bad userdata")
	region   = flag.String("region", "us-east-1", "aws region")
	instance = flag.String("instance", "", "aws instance to generate new user-data")
)

// UserData contains the current user-data and the parsed coreos cloudinit
type UserData struct {
	Current   string
	CloudInit *config.CloudConfig
}

func main() {
	flag.Parse()
	if *all == false && *instance == "" {
		flag.Usage()
		os.Exit(1)
	}

	if *instance != "" {
		os.Exit(singleInstance(*region, *instance))
	}
	os.Exit(allInstances())
}

// singleInstance prints out the diff change, and, if user wants,
// prints the modified user data. Returns exit code
func singleInstance(region, instance string) int {
	ud := userData(region, instance)
	conf := update(ud.CloudInit)
	if conf == nil {
		return 1
	}

	dmp := diffmatchpatch.New()
	diffs := dmp.DiffMain(ud.Current, conf.String(), false)
	fmt.Println(dmp.DiffPrettyText(diffs))

	reader := bufio.NewReader(os.Stdin)
	fmt.Println("if diff looks right, type y (user-data will follow)")
	text, _ := reader.ReadString('\n')
	if text == "y\n" {
		fmt.Printf("New user data:\n\n\n\n\n\n\n%s", conf.String())
	}
	return 0
}

func allInstances() int {
	regionInsts, err := instances()
	if err != nil {
		fmt.Println(err)
		return 1
	}

	for region, insts := range regionInsts {
		for _, inst := range insts {
			ud := userData(region, inst)
			if ud == nil {
				continue
			}
			if init := update(ud.CloudInit); init != nil {
				fmt.Printf("%s %s\n", region, inst)
			}
			time.Sleep(time.Second)
		}
	}
	return 0
}

func userData(region, instance string) *UserData {
	cfg, err := external.LoadDefaultAWSConfig()
	if err != nil {
		log.Printf(err.Error())
		return nil
	}

	cfg.Region = region
	svc := ec2.New(cfg)
	input := &ec2.DescribeInstanceAttributeInput{
		Attribute:  "userData",
		InstanceId: aws.String(instance),
	}

	req := svc.DescribeInstanceAttributeRequest(input)
	res, err := req.Send()
	if err != nil {
		log.Printf(err.Error())
		return nil
	}

	if res.UserData == nil || res.UserData.Value == nil {
		log.Printf("no user data")
		return nil
	}
	data, err := base64.StdEncoding.DecodeString(*res.UserData.Value)
	if err != nil {
		panic(err)
	}

	conf, err := config.NewCloudConfig(string(data))
	if err != nil {
		log.Printf("no user data")
		return nil
	}
	return &UserData{
		Current:   string(data),
		CloudInit: conf,
	}
}

func instances() (map[string][]string, error) {
	cfg, err := external.LoadDefaultAWSConfig()
	if err != nil {
		return nil, err
	}

	partition := endpoints.AwsPartition()
	regions := partition.Regions()
	regionInsts := map[string][]string{}
	for r := range regions {
		insts := []string{}
		cfg.Region = r
		svc := ec2.New(cfg)

		input := &ec2.DescribeInstancesInput{
			Filters: []ec2.Filter{
				ec2.Filter{
					Name:   aws.String("instance-state-name"),
					Values: []string{"running", "pending"},
				},
			},
		}

		req := svc.DescribeInstancesRequest(input)
		p := req.Paginate()
		for p.Next() {
			res := p.CurrentPage()
			if res == nil {
				continue
			}
			for _, reservation := range res.Reservations {
				for _, instance := range reservation.Instances {
					insts = append(insts, *instance.InstanceId)
				}
			}
		}

		if err := p.Err(); err != nil {
			return nil, err
		}

		regionInsts[r] = insts
	}
	return regionInsts, nil
}

func update(conf *config.CloudConfig) *config.CloudConfig {
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
		return nil
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
	return conf
}
