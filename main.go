package main

import (
	"fmt"
	"github.com/codegangsta/cli"
	"github.com/codius/montecarlo"
	"github.com/google/go-github/github"
	"github.com/mgutz/ansi"
	"golang.org/x/oauth2"
	"gopkg.in/redis.v3"
	"net/url"
	"os"
	"strconv"
	"strings"
)

type tokenSource struct {
	token *oauth2.Token
}

func (t *tokenSource) Token() (*oauth2.Token, error) {
	return t.token, nil
}

func printConditions(conditions []monty.Condition, depth int) {
	for _, condition := range conditions {
		var result string
		if condition.Passed {
			result = ansi.Color(condition.Message, "green")
		} else {
			result = ansi.Color(condition.Message, "red")
		}
		fmt.Printf("%v%v:\t%v\n", strings.Repeat("\t", depth), condition.Name, result)
		printConditions(condition.Subconditions, depth+1)
	}
}

func main() {

	token := os.Getenv("GITHUB_TOKEN")

	if len(token) == 0 {
		panic("No github token")
	}

	ts := &tokenSource{
		&oauth2.Token{AccessToken: token},
	}

	tc := oauth2.NewClient(oauth2.NoContext, ts)
	client := github.NewClient(tc)

	redisURL, err := url.Parse(os.Getenv("REDISCLOUD_URL"))

	if err != nil || len(os.Getenv("REDISCLOUD_URL")) == 0 {
		panic("REDISCLOUD_URL not set")
	}

	addr := redisURL.Host
	passwd, _ := redisURL.User.Password()

	options := redis.Options{
		Addr:     addr,
		Password: passwd,
	}

	robot := monty.NewBrain(client, &options)

	app := cli.NewApp()

	app.Name = "montecarlo"
	app.Commands = []cli.Command{
		{
			Name:  "sync",
			Usage: "Synchronises local state with github",
			Action: func(c *cli.Context) {
				robot.SyncRepositories()
			},
		},
		{
			Name:  "review",
			Usage: "Reviews and merges open pull requests",
			Action: func(c *cli.Context) {
				reviews := robot.ReviewPRs()
				for _, review := range reviews {
					if review.Condition.Passed {
						fmt.Printf("Merging %s!\n", review)
					}
				}
			},
		},
		{
			Name:  "dashboard",
			Usage: "Serves report data through a local REST api",
			Action: func(c *cli.Context) {
				port := 8080

				if len(os.Getenv("PORT")) > 0 {
					port, _ = strconv.Atoi(os.Getenv("PORT"))
				}

				monty.NewRestServer(robot, port).Run()
			},
		},
		{
			Name:  "status",
			Usage: "Reports status of open pull requests",
			Action: func(c *cli.Context) {
				reviews := robot.ReviewPRs()
				for _, review := range reviews {
					fmt.Printf("%v/%v - %v\n", *review.Repository.FullName, review.PullRequest.Number, review.PullRequest.Title)
					conditions := make([]monty.Condition, 1)
					conditions[0] = review.Condition
					printConditions(conditions, 1)
				}
			},
		},
	}

	app.Run(os.Args)
}
