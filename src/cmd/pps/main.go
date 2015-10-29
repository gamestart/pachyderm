package main

import (
	"fmt"
	"os"
	"strings"
	"text/tabwriter"

	"golang.org/x/net/context"

	"github.com/pachyderm/pachyderm/src/pfs"
	"github.com/pachyderm/pachyderm/src/pps"
	"github.com/pachyderm/pachyderm/src/pps/pretty"

	"go.pedge.io/env"
	"go.pedge.io/pkg/cobra"

	"google.golang.org/grpc"

	"github.com/spf13/cobra"
)

var (
	defaultEnv = map[string]string{
		"PPS_ADDRESS": "0.0.0.0:651",
	}
)

type appEnv struct {
	PachydermPpsd1Port string `env:"PACHYDERM_PPSD_1_PORT"`
	Address            string `env:"PPS_ADDRESS"`
}

func main() {
	env.Main(do, &appEnv{}, defaultEnv)
}

func do(appEnvObj interface{}) error {
	appEnv := appEnvObj.(*appEnv)
	address := appEnv.PachydermPpsd1Port
	if address == "" {
		address = appEnv.Address
	} else {
		address = strings.Replace(address, "tcp://", "", -1)
	}
	clientConn, err := grpc.Dial(address, grpc.WithInsecure())
	if err != nil {
		return err
	}
	apiClient := pps.NewJobAPIClient(clientConn)
	rootCmd := &cobra.Command{
		Use: "pps",
		Long: `Access the PPS API.

Note that this CLI is experimental and does not even check for common errors.
The environment variable PPS_ADDRESS controls what server the CLI connects to, the default is 0.0.0.0:651.`,
	}

	var image string
	var outParentCommitId string
	createJob := &cobra.Command{
		Use:   "create-job in-repo-name in-commit-id out-repo-name -i image -p out-parent-commit-id command [args]",
		Short: "Create a new job. Returns the id of the created job.",
		Long: `Create a new job. With repo-name/commit-id as input and
out-repo-name as output. A commit will be created for the output.
You can find out the name of the commit with inspect-job.`,
		Run: func(cmd *cobra.Command, args []string) {
			job, err := apiClient.CreateJob(
				context.Background(),
				&pps.CreateJobRequest{
					Spec: &pps.CreateJobRequest_Transform{
						Transform: &pps.Transform{
							Image: image,
							Cmd:   args[3:],
						},
					},
					Input: &pfs.Commit{
						Repo: &pfs.Repo{
							Name: args[0],
						},
						Id: args[1],
					},
					OutputParent: &pfs.Commit{
						Repo: &pfs.Repo{
							Name: args[2],
						},
						Id: outParentCommitId,
					},
				})
			if err != nil {
				errorAndExit("Error from CreateJob: %s", err.Error())
			}
			fmt.Println(job.Id)
		},
	}
	createJob.Flags().StringVarP(&image, "image", "i", "ubuntu", "The image to run the job in.")
	createJob.Flags().StringVarP(&outParentCommitId, "parent", "p", "", "The parent to use for the output commit.")

	inspectJob := &cobra.Command{
		Use:   "inspect-job job-id",
		Short: "Return info about a job.",
		Long:  "Return info about a job.",
		Run: pkgcobra.RunFixedArgs(1, func(args []string) error {
			jobInfo, err := apiClient.InspectJob(
				context.Background(),
				&pps.InspectJobRequest{
					Job: &pps.Job{
						Id: args[0],
					},
				},
			)
			if err != nil {
				errorAndExit("Error from InspectJob: %s", err.Error())
			}
			writer := tabwriter.NewWriter(os.Stdout, 20, 1, 3, ' ', 0)
			pretty.PrintJobHeader(writer)
			pretty.PrintJobInfo(writer, jobInfo)
			return writer.Flush()
			return nil
		}),
	}

	rootCmd.AddCommand(createJob)
	rootCmd.AddCommand(inspectJob)
	return rootCmd.Execute()
}

func errorAndExit(format string, args ...interface{}) {
	fmt.Fprintf(os.Stderr, "%s\n", fmt.Sprintf(format, args...))
	os.Exit(1)
}
