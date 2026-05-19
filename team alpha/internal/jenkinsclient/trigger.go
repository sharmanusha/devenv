package jenkinsclient

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"os/exec"
	"strings"
	"time"
)

type JenkinsCrumb struct {
	Crumb             string `json:"crumb"`
	CrumbRequestField string `json:"crumbRequestField"`
}

// TriggerIntegratedBuild starts devenv/local-ci-cd with git and app parameters.
func TriggerIntegratedBuild(
	jenkinsBaseURL,
	gitURL,
	branch,
	appName string,
) error {

	gitURL = strings.TrimSpace(gitURL)

	if gitURL == "" {
		return fmt.Errorf(
			"GIT_URL is required for Jenkins builds — use devenv run for uncommitted local changes",
		)
	}

	if branch == "" {
		branch = "main"
	}

	base := strings.TrimSuffix(
		strings.TrimSpace(jenkinsBaseURL),
		"/",
	)

	jobURL := fmt.Sprintf(
		"%s/job/devenv/job/local-ci-cd/buildWithParameters",
		base,
	)

	form := url.Values{}
	form.Set("GIT_URL", gitURL)
	form.Set("GIT_BRANCH", branch)

	if strings.TrimSpace(appName) != "" {
		form.Set("APP_NAME", strings.TrimSpace(appName))
	}

	jar, _ := cookiejar.New(nil)

	client := &http.Client{
		Jar:     jar,
		Timeout: 30 * time.Second,
	}

	// ---------------------------------------------------
	// FETCH CRUMB
	// ---------------------------------------------------

	crumbReq, err := http.NewRequest(
		http.MethodGet,
		base+"/crumbIssuer/api/json",
		nil,
	)
	if err != nil {
		return err
	}

	crumbReq.SetBasicAuth("admin", "admin123")

	crumbResp, err := client.Do(crumbReq)
	if err != nil {
		return err
	}
	defer crumbResp.Body.Close()

	var crumb JenkinsCrumb

	if err := json.NewDecoder(crumbResp.Body).Decode(&crumb); err != nil {
		return err
	}

	// ---------------------------------------------------
	// TRIGGER BUILD
	// ---------------------------------------------------

	req, err := http.NewRequest(
		http.MethodPost,
		jobURL,
		strings.NewReader(form.Encode()),
	)
	if err != nil {
		return err
	}

	req.SetBasicAuth("admin", "admin123")

	req.Header.Set(
		crumb.CrumbRequestField,
		crumb.Crumb,
	)

	req.Header.Set(
		"Content-Type",
		"application/x-www-form-urlencoded",
	)

	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode == 201 ||
		resp.StatusCode == 200 ||
		resp.StatusCode == 302 {

		fmt.Println(
			"[OK] Jenkins build queued: devenv/local-ci-cd",
		)

		fmt.Printf(
			"[INFO] Track progress: %s/job/devenv/job/local-ci-cd/\n",
			base,
		)

		return nil
	}

	body, _ := io.ReadAll(resp.Body)

	return fmt.Errorf(
		"jenkins trigger HTTP %d: %s",
		resp.StatusCode,
		strings.TrimSpace(string(body)),
	)
}

// EnsureJenkinsJob auto-creates Jenkins folder + pipeline.
func EnsureJenkinsJob(base string) error {

	base = strings.TrimSuffix(strings.TrimSpace(base), "/")

	jar, _ := cookiejar.New(nil)

	client := &http.Client{
		Jar:     jar,
		Timeout: 30 * time.Second,
	}

	// ---------------------------------------------------
	// FETCH CRUMB
	// ---------------------------------------------------

	crumbReq, err := http.NewRequest(
		http.MethodGet,
		base+"/crumbIssuer/api/json",
		nil,
	)
	if err != nil {
		return err
	}

	crumbReq.SetBasicAuth("admin", "admin123")

	crumbResp, err := client.Do(crumbReq)
	if err != nil {
		return err
	}
	defer crumbResp.Body.Close()

	var crumb JenkinsCrumb

	if err := json.NewDecoder(crumbResp.Body).Decode(&crumb); err != nil {
		return err
	}

	// ---------------------------------------------------
	// CHECK IF FOLDER ALREADY EXISTS
	// ---------------------------------------------------

	checkFolderReq, err := http.NewRequest(
		http.MethodGet,
		base+"/job/devenv/api/json",
		nil,
	)
	if err != nil {
		return err
	}

	checkFolderReq.SetBasicAuth("admin", "admin123")

	checkFolderResp, err := client.Do(checkFolderReq)

	if err == nil && checkFolderResp.StatusCode == 200 {

		fmt.Println("[INFO] Jenkins folder already exists")

	} else {

		// ---------------------------------------------------
		// CREATE FOLDER
		// ---------------------------------------------------

		folderURL := fmt.Sprintf(
			"%s/createItem?name=devenv",
			base,
		)

		folderXML := `<com.cloudbees.hudson.plugins.folder.Folder plugin="cloudbees-folder"></com.cloudbees.hudson.plugins.folder.Folder>`

		folderReq, err := http.NewRequest(
			http.MethodPost,
			folderURL,
			strings.NewReader(folderXML),
		)
		if err != nil {
			return err
		}

		folderReq.SetBasicAuth("admin", "admin123")

		folderReq.Header.Set(
			crumb.CrumbRequestField,
			crumb.Crumb,
		)

		folderReq.Header.Set(
			"Content-Type",
			"application/xml",
		)

		folderResp, err := client.Do(folderReq)
		if err != nil {
			return err
		}
		defer folderResp.Body.Close()

		fmt.Println("[OK] Jenkins folder created")
	}

	// ---------------------------------------------------
	// CHECK IF PIPELINE ALREADY EXISTS
	// ---------------------------------------------------

	checkJobReq, err := http.NewRequest(
		http.MethodGet,
		base+"/job/devenv/job/local-ci-cd/api/json",
		nil,
	)
	if err != nil {
		return err
	}

	checkJobReq.SetBasicAuth("admin", "admin123")

	checkJobResp, err := client.Do(checkJobReq)

	if err == nil && checkJobResp.StatusCode == 200 {

		fmt.Println("[INFO] Jenkins pipeline already exists")
		return nil
	}

	// ---------------------------------------------------
	// CREATE PIPELINE
	// ---------------------------------------------------

	jobURL := fmt.Sprintf(
		"%s/job/devenv/createItem?name=local-ci-cd",
		base,
	)

	jobXML := `<?xml version='1.1' encoding='UTF-8'?>
<flow-definition plugin="workflow-job">
  <actions/>
  <description>Local CI/CD Pipeline</description>
  <keepDependencies>false</keepDependencies>

  <properties>
    <hudson.model.ParametersDefinitionProperty>
      <parameterDefinitions>

        <hudson.model.StringParameterDefinition>
          <name>GIT_URL</name>
          <defaultValue></defaultValue>
          <trim>true</trim>
        </hudson.model.StringParameterDefinition>

        <hudson.model.StringParameterDefinition>
          <name>GIT_BRANCH</name>
          <defaultValue>main</defaultValue>
          <trim>true</trim>
        </hudson.model.StringParameterDefinition>

        <hudson.model.StringParameterDefinition>
          <name>APP_NAME</name>
          <defaultValue></defaultValue>
          <trim>true</trim>
        </hudson.model.StringParameterDefinition>

      </parameterDefinitions>
    </hudson.model.ParametersDefinitionProperty>
  </properties>

  <definition class="org.jenkinsci.plugins.workflow.cps.CpsFlowDefinition">
    <script>
pipeline {
    agent any

    parameters {
        string(name: 'GIT_URL', defaultValue: '', description: 'Git repository URL')
        string(name: 'GIT_BRANCH', defaultValue: 'main', description: 'Git branch')
        string(name: 'APP_NAME', defaultValue: '', description: 'Application name')
    }

    stages {

        stage('Build') {
            steps {
                sh 'echo Build successful'
            }
        }

        stage('Deploy') {
            steps {
                sh 'echo Deploy successful'
            }
        }
    }
}
    </script>

    <sandbox>true</sandbox>
  </definition>

  <triggers/>
  <disabled>false</disabled>
</flow-definition>`

	jobReq, err := http.NewRequest(
		http.MethodPost,
		jobURL,
		strings.NewReader(jobXML),
	)
	if err != nil {
		return err
	}

	jobReq.SetBasicAuth("admin", "admin123")

	jobReq.Header.Set(
		crumb.CrumbRequestField,
		crumb.Crumb,
	)

	jobReq.Header.Set(
		"Content-Type",
		"application/xml",
	)

	jobResp, err := client.Do(jobReq)
	if err != nil {
		return err
	}
	defer jobResp.Body.Close()

	if jobResp.StatusCode == 200 ||
		jobResp.StatusCode == 201 ||
		jobResp.StatusCode == 400 {

		fmt.Println("[OK] Jenkins pipeline ready")
		return nil
	}

	body, _ := io.ReadAll(jobResp.Body)

	return fmt.Errorf(
		"jenkins job creation failed: %s",
		string(body),
	)
}

// GitRemoteOrigin returns origin URL for a project directory, if configured.
func GitRemoteOrigin(projectPath string) (string, error) {

	out, err := exec.Command(
		"git",
		"-C",
		projectPath,
		"remote",
		"get-url",
		"origin",
	).Output()

	if err != nil {
		return "", err
	}

	return strings.TrimSpace(string(out)), nil
}

// JenkinsURLFromState reads Jenkins URL from platform configmap (cluster) or defaults.
func JenkinsURLFromState() string {

	out, err := exec.Command(
		"kubectl",
		"get",
		"configmap",
		"devenv-platform-config",
		"-n",
		"devenv-system",
		"-o",
		"jsonpath={.data.platform\\.json}",
	).Output()

	if err != nil {
		return "http://127.0.0.1:8080"
	}

	var cfg struct {
		JenkinsURL string `json:"jenkins_url"`
	}

	if json.Unmarshal(out, &cfg) == nil &&
		cfg.JenkinsURL != "" {

		return cfg.JenkinsURL
	}

	return "http://127.0.0.1:8080"
}