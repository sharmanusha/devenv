package cmd

import (
	"devenv/teamalpha/orchestrator"

	"github.com/spf13/cobra"
)

var useJenkins bool

var runCmd = &cobra.Command{
	Use:   "run",
	Short: "Run the full local CI/CD flow",
	Long: `Run the integrated local CI/CD pipeline.

By default devenv runs the pipeline on your machine (build, push, deploy) — best for
saved UI changes without git commit.

Use --jenkins to trigger the shared Jenkins job (devenv/local-ci-cd) which uses the
same registry and cluster; requires git remote origin and pushed commits.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		return orchestrator.RunPipelineOptions(orchestrator.PipelineOptions{
			UseJenkins: useJenkins,
		})
	},
}

func init() {
	runCmd.Flags().BoolVar(&useJenkins, "jenkins", false, "Trigger integrated Jenkins job instead of host pipeline")
	rootCmd.AddCommand(runCmd)
}
