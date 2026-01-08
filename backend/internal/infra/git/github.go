package git

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

// GitHubProvider implements Provider interface for GitHub
type GitHubProvider struct {
	baseURL     string
	accessToken string
	httpClient  *http.Client
}

// NewGitHubProvider creates a new GitHub provider
func NewGitHubProvider(baseURL, accessToken string) (*GitHubProvider, error) {
	if baseURL == "" {
		baseURL = "https://api.github.com"
	}
	baseURL = strings.TrimSuffix(baseURL, "/")

	return &GitHubProvider{
		baseURL:     baseURL,
		accessToken: accessToken,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}, nil
}

func (p *GitHubProvider) doRequest(ctx context.Context, method, path string, body io.Reader) (*http.Response, error) {
	reqURL := p.baseURL + path

	req, err := http.NewRequestWithContext(ctx, method, reqURL, body)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Authorization", "Bearer "+p.accessToken)
	req.Header.Set("Accept", "application/vnd.github.v3+json")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode == http.StatusUnauthorized {
		resp.Body.Close()
		return nil, ErrUnauthorized
	}
	if resp.StatusCode == http.StatusNotFound {
		resp.Body.Close()
		return nil, ErrNotFound
	}
	if resp.StatusCode == http.StatusForbidden {
		resp.Body.Close()
		return nil, ErrRateLimited
	}

	return resp, nil
}

// GetCurrentUser returns the authenticated user
func (p *GitHubProvider) GetCurrentUser(ctx context.Context) (*User, error) {
	resp, err := p.doRequest(ctx, http.MethodGet, "/user", nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var ghUser struct {
		ID        int    `json:"id"`
		Login     string `json:"login"`
		Name      string `json:"name"`
		Email     string `json:"email"`
		AvatarURL string `json:"avatar_url"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&ghUser); err != nil {
		return nil, err
	}

	return &User{
		ID:        strconv.Itoa(ghUser.ID),
		Username:  ghUser.Login,
		Name:      ghUser.Name,
		Email:     ghUser.Email,
		AvatarURL: ghUser.AvatarURL,
	}, nil
}

// GetProject returns repository details
func (p *GitHubProvider) GetProject(ctx context.Context, projectID string) (*Project, error) {
	// projectID format: owner/repo
	resp, err := p.doRequest(ctx, http.MethodGet, fmt.Sprintf("/repos/%s", projectID), nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var ghRepo struct {
		ID            int       `json:"id"`
		Name          string    `json:"name"`
		FullName      string    `json:"full_name"`
		Description   string    `json:"description"`
		DefaultBranch string    `json:"default_branch"`
		HTMLURL       string    `json:"html_url"`
		CloneURL      string    `json:"clone_url"`
		SSHURL        string    `json:"ssh_url"`
		Visibility    string    `json:"visibility"`
		CreatedAt     time.Time `json:"created_at"`
		UpdatedAt     time.Time `json:"updated_at"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&ghRepo); err != nil {
		return nil, err
	}

	return &Project{
		ID:            strconv.Itoa(ghRepo.ID),
		Name:          ghRepo.Name,
		FullPath:      ghRepo.FullName,
		Description:   ghRepo.Description,
		DefaultBranch: ghRepo.DefaultBranch,
		WebURL:        ghRepo.HTMLURL,
		CloneURL:      ghRepo.CloneURL,
		SSHCloneURL:   ghRepo.SSHURL,
		Visibility:    ghRepo.Visibility,
		CreatedAt:     ghRepo.CreatedAt,
		UpdatedAt:     ghRepo.UpdatedAt,
	}, nil
}

// ListProjects returns user's repositories
func (p *GitHubProvider) ListProjects(ctx context.Context, page, perPage int) ([]*Project, error) {
	path := fmt.Sprintf("/user/repos?page=%d&per_page=%d&sort=updated", page, perPage)
	resp, err := p.doRequest(ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var ghRepos []struct {
		ID            int       `json:"id"`
		Name          string    `json:"name"`
		FullName      string    `json:"full_name"`
		Description   string    `json:"description"`
		DefaultBranch string    `json:"default_branch"`
		HTMLURL       string    `json:"html_url"`
		CloneURL      string    `json:"clone_url"`
		SSHURL        string    `json:"ssh_url"`
		Visibility    string    `json:"visibility"`
		CreatedAt     time.Time `json:"created_at"`
		UpdatedAt     time.Time `json:"updated_at"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&ghRepos); err != nil {
		return nil, err
	}

	projects := make([]*Project, len(ghRepos))
	for i, ghr := range ghRepos {
		projects[i] = &Project{
			ID:            strconv.Itoa(ghr.ID),
			Name:          ghr.Name,
			FullPath:      ghr.FullName,
			Description:   ghr.Description,
			DefaultBranch: ghr.DefaultBranch,
			WebURL:        ghr.HTMLURL,
			CloneURL:      ghr.CloneURL,
			SSHCloneURL:   ghr.SSHURL,
			Visibility:    ghr.Visibility,
			CreatedAt:     ghr.CreatedAt,
			UpdatedAt:     ghr.UpdatedAt,
		}
	}

	return projects, nil
}

// SearchProjects searches for repositories
func (p *GitHubProvider) SearchProjects(ctx context.Context, query string, page, perPage int) ([]*Project, error) {
	path := fmt.Sprintf("/search/repositories?q=%s&page=%d&per_page=%d", url.QueryEscape(query), page, perPage)
	resp, err := p.doRequest(ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result struct {
		Items []struct {
			ID            int       `json:"id"`
			Name          string    `json:"name"`
			FullName      string    `json:"full_name"`
			Description   string    `json:"description"`
			DefaultBranch string    `json:"default_branch"`
			HTMLURL       string    `json:"html_url"`
			CloneURL      string    `json:"clone_url"`
			SSHURL        string    `json:"ssh_url"`
			Visibility    string    `json:"visibility"`
			CreatedAt     time.Time `json:"created_at"`
			UpdatedAt     time.Time `json:"updated_at"`
		} `json:"items"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	projects := make([]*Project, len(result.Items))
	for i, ghr := range result.Items {
		projects[i] = &Project{
			ID:            strconv.Itoa(ghr.ID),
			Name:          ghr.Name,
			FullPath:      ghr.FullName,
			Description:   ghr.Description,
			DefaultBranch: ghr.DefaultBranch,
			WebURL:        ghr.HTMLURL,
			CloneURL:      ghr.CloneURL,
			SSHCloneURL:   ghr.SSHURL,
			Visibility:    ghr.Visibility,
			CreatedAt:     ghr.CreatedAt,
			UpdatedAt:     ghr.UpdatedAt,
		}
	}

	return projects, nil
}

// ListBranches returns branches for a repository
func (p *GitHubProvider) ListBranches(ctx context.Context, projectID string) ([]*Branch, error) {
	resp, err := p.doRequest(ctx, http.MethodGet, fmt.Sprintf("/repos/%s/branches", projectID), nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var ghBranches []struct {
		Name      string `json:"name"`
		Commit    struct {
			SHA string `json:"sha"`
		} `json:"commit"`
		Protected bool `json:"protected"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&ghBranches); err != nil {
		return nil, err
	}

	// Get default branch
	project, _ := p.GetProject(ctx, projectID)
	defaultBranch := ""
	if project != nil {
		defaultBranch = project.DefaultBranch
	}

	branches := make([]*Branch, len(ghBranches))
	for i, ghb := range ghBranches {
		branches[i] = &Branch{
			Name:      ghb.Name,
			CommitSHA: ghb.Commit.SHA,
			Protected: ghb.Protected,
			Default:   ghb.Name == defaultBranch,
		}
	}

	return branches, nil
}

// GetBranch returns a specific branch
func (p *GitHubProvider) GetBranch(ctx context.Context, projectID, branchName string) (*Branch, error) {
	resp, err := p.doRequest(ctx, http.MethodGet, fmt.Sprintf("/repos/%s/branches/%s", projectID, url.PathEscape(branchName)), nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var ghBranch struct {
		Name      string `json:"name"`
		Commit    struct {
			SHA string `json:"sha"`
		} `json:"commit"`
		Protected bool `json:"protected"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&ghBranch); err != nil {
		return nil, err
	}

	project, _ := p.GetProject(ctx, projectID)
	isDefault := project != nil && ghBranch.Name == project.DefaultBranch

	return &Branch{
		Name:      ghBranch.Name,
		CommitSHA: ghBranch.Commit.SHA,
		Protected: ghBranch.Protected,
		Default:   isDefault,
	}, nil
}

// CreateBranch creates a new branch
func (p *GitHubProvider) CreateBranch(ctx context.Context, projectID, branchName, ref string) (*Branch, error) {
	// First get the SHA of the ref
	refResp, err := p.doRequest(ctx, http.MethodGet, fmt.Sprintf("/repos/%s/git/refs/heads/%s", projectID, ref), nil)
	if err != nil {
		return nil, err
	}

	var refData struct {
		Object struct {
			SHA string `json:"sha"`
		} `json:"object"`
	}
	json.NewDecoder(refResp.Body).Decode(&refData)
	refResp.Body.Close()

	// Create the new branch
	body := fmt.Sprintf(`{"ref":"refs/heads/%s","sha":"%s"}`, branchName, refData.Object.SHA)
	resp, err := p.doRequest(ctx, http.MethodPost, fmt.Sprintf("/repos/%s/git/refs", projectID), strings.NewReader(body))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	return &Branch{
		Name:      branchName,
		CommitSHA: refData.Object.SHA,
		Protected: false,
		Default:   false,
	}, nil
}

// DeleteBranch deletes a branch
func (p *GitHubProvider) DeleteBranch(ctx context.Context, projectID, branchName string) error {
	resp, err := p.doRequest(ctx, http.MethodDelete, fmt.Sprintf("/repos/%s/git/refs/heads/%s", projectID, url.PathEscape(branchName)), nil)
	if err != nil {
		return err
	}
	resp.Body.Close()
	return nil
}

// GetMergeRequest returns a specific pull request
func (p *GitHubProvider) GetMergeRequest(ctx context.Context, projectID string, mrIID int) (*MergeRequest, error) {
	resp, err := p.doRequest(ctx, http.MethodGet, fmt.Sprintf("/repos/%s/pulls/%d", projectID, mrIID), nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	return p.parsePullRequest(resp.Body)
}

// ListMergeRequests returns pull requests for a repository
func (p *GitHubProvider) ListMergeRequests(ctx context.Context, projectID string, state string, page, perPage int) ([]*MergeRequest, error) {
	ghState := "all"
	if state == "opened" {
		ghState = "open"
	} else if state == "merged" || state == "closed" {
		ghState = "closed"
	}

	path := fmt.Sprintf("/repos/%s/pulls?state=%s&page=%d&per_page=%d", projectID, ghState, page, perPage)
	resp, err := p.doRequest(ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var ghPRs []struct {
		ID        int    `json:"id"`
		Number    int    `json:"number"`
		Title     string `json:"title"`
		Body      string `json:"body"`
		Head      struct {
			Ref string `json:"ref"`
		} `json:"head"`
		Base struct {
			Ref string `json:"ref"`
		} `json:"base"`
		State   string `json:"state"`
		HTMLURL string `json:"html_url"`
		User    struct {
			ID        int    `json:"id"`
			Login     string `json:"login"`
			AvatarURL string `json:"avatar_url"`
		} `json:"user"`
		CreatedAt time.Time  `json:"created_at"`
		UpdatedAt time.Time  `json:"updated_at"`
		MergedAt  *time.Time `json:"merged_at"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&ghPRs); err != nil {
		return nil, err
	}

	mrs := make([]*MergeRequest, len(ghPRs))
	for i, ghPR := range ghPRs {
		state := ghPR.State
		if ghPR.MergedAt != nil {
			state = "merged"
		}

		mrs[i] = &MergeRequest{
			ID:           ghPR.ID,
			IID:          ghPR.Number,
			Title:        ghPR.Title,
			Description:  ghPR.Body,
			SourceBranch: ghPR.Head.Ref,
			TargetBranch: ghPR.Base.Ref,
			State:        state,
			WebURL:       ghPR.HTMLURL,
			Author: &User{
				ID:        strconv.Itoa(ghPR.User.ID),
				Username:  ghPR.User.Login,
				AvatarURL: ghPR.User.AvatarURL,
			},
			CreatedAt: ghPR.CreatedAt,
			UpdatedAt: ghPR.UpdatedAt,
			MergedAt:  ghPR.MergedAt,
		}
	}

	return mrs, nil
}

// ListMergeRequestsByBranch returns pull requests filtered by head branch
func (p *GitHubProvider) ListMergeRequestsByBranch(ctx context.Context, projectID, sourceBranch, state string) ([]*MergeRequest, error) {
	ghState := "all"
	if state == "opened" {
		ghState = "open"
	} else if state == "merged" || state == "closed" {
		ghState = "closed"
	}

	// GitHub requires owner:branch format for head parameter when filtering across forks
	path := fmt.Sprintf("/repos/%s/pulls?state=%s&head=%s", projectID, ghState, url.QueryEscape(sourceBranch))
	resp, err := p.doRequest(ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var ghPRs []struct {
		ID        int    `json:"id"`
		Number    int    `json:"number"`
		Title     string `json:"title"`
		Body      string `json:"body"`
		Head      struct {
			Ref string `json:"ref"`
		} `json:"head"`
		Base struct {
			Ref string `json:"ref"`
		} `json:"base"`
		State          string     `json:"state"`
		HTMLURL        string     `json:"html_url"`
		MergeCommitSHA string     `json:"merge_commit_sha"`
		MergedAt       *time.Time `json:"merged_at"`
		User           struct {
			ID        int    `json:"id"`
			Login     string `json:"login"`
			AvatarURL string `json:"avatar_url"`
		} `json:"user"`
		CreatedAt time.Time `json:"created_at"`
		UpdatedAt time.Time `json:"updated_at"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&ghPRs); err != nil {
		return nil, err
	}

	mrs := make([]*MergeRequest, len(ghPRs))
	for i, ghPR := range ghPRs {
		prState := ghPR.State
		if ghPR.MergedAt != nil {
			prState = "merged"
		}

		mrs[i] = &MergeRequest{
			ID:             ghPR.ID,
			IID:            ghPR.Number,
			Title:          ghPR.Title,
			Description:    ghPR.Body,
			SourceBranch:   ghPR.Head.Ref,
			TargetBranch:   ghPR.Base.Ref,
			State:          prState,
			WebURL:         ghPR.HTMLURL,
			MergeCommitSHA: ghPR.MergeCommitSHA,
			MergedAt:       ghPR.MergedAt,
			Author: &User{
				ID:        strconv.Itoa(ghPR.User.ID),
				Username:  ghPR.User.Login,
				AvatarURL: ghPR.User.AvatarURL,
			},
			CreatedAt: ghPR.CreatedAt,
			UpdatedAt: ghPR.UpdatedAt,
		}
	}

	return mrs, nil
}

// CreateMergeRequest creates a new pull request
func (p *GitHubProvider) CreateMergeRequest(ctx context.Context, req *CreateMRRequest) (*MergeRequest, error) {
	body := fmt.Sprintf(`{"title":"%s","body":"%s","head":"%s","base":"%s"}`,
		req.Title, req.Description, req.SourceBranch, req.TargetBranch)

	resp, err := p.doRequest(ctx, http.MethodPost, fmt.Sprintf("/repos/%s/pulls", req.ProjectID), strings.NewReader(body))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	return p.parsePullRequest(resp.Body)
}

// UpdateMergeRequest updates a pull request
func (p *GitHubProvider) UpdateMergeRequest(ctx context.Context, projectID string, mrIID int, title, description string) (*MergeRequest, error) {
	body := fmt.Sprintf(`{"title":"%s","body":"%s"}`, title, description)

	resp, err := p.doRequest(ctx, http.MethodPatch, fmt.Sprintf("/repos/%s/pulls/%d", projectID, mrIID), strings.NewReader(body))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	return p.parsePullRequest(resp.Body)
}

// MergeMergeRequest merges a pull request
func (p *GitHubProvider) MergeMergeRequest(ctx context.Context, projectID string, mrIID int) (*MergeRequest, error) {
	resp, err := p.doRequest(ctx, http.MethodPut, fmt.Sprintf("/repos/%s/pulls/%d/merge", projectID, mrIID), nil)
	if err != nil {
		return nil, err
	}
	resp.Body.Close()

	return p.GetMergeRequest(ctx, projectID, mrIID)
}

// CloseMergeRequest closes a pull request
func (p *GitHubProvider) CloseMergeRequest(ctx context.Context, projectID string, mrIID int) (*MergeRequest, error) {
	body := `{"state":"closed"}`

	resp, err := p.doRequest(ctx, http.MethodPatch, fmt.Sprintf("/repos/%s/pulls/%d", projectID, mrIID), strings.NewReader(body))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	return p.parsePullRequest(resp.Body)
}

func (p *GitHubProvider) parsePullRequest(r io.Reader) (*MergeRequest, error) {
	var ghPR struct {
		ID        int    `json:"id"`
		Number    int    `json:"number"`
		Title     string `json:"title"`
		Body      string `json:"body"`
		Head      struct {
			Ref string `json:"ref"`
		} `json:"head"`
		Base struct {
			Ref string `json:"ref"`
		} `json:"base"`
		State   string `json:"state"`
		HTMLURL string `json:"html_url"`
		User    struct {
			ID        int    `json:"id"`
			Login     string `json:"login"`
			AvatarURL string `json:"avatar_url"`
		} `json:"user"`
		CreatedAt time.Time  `json:"created_at"`
		UpdatedAt time.Time  `json:"updated_at"`
		MergedAt  *time.Time `json:"merged_at"`
	}

	if err := json.NewDecoder(r).Decode(&ghPR); err != nil {
		return nil, err
	}

	state := ghPR.State
	if ghPR.MergedAt != nil {
		state = "merged"
	}

	return &MergeRequest{
		ID:           ghPR.ID,
		IID:          ghPR.Number,
		Title:        ghPR.Title,
		Description:  ghPR.Body,
		SourceBranch: ghPR.Head.Ref,
		TargetBranch: ghPR.Base.Ref,
		State:        state,
		WebURL:       ghPR.HTMLURL,
		Author: &User{
			ID:        strconv.Itoa(ghPR.User.ID),
			Username:  ghPR.User.Login,
			AvatarURL: ghPR.User.AvatarURL,
		},
		CreatedAt: ghPR.CreatedAt,
		UpdatedAt: ghPR.UpdatedAt,
		MergedAt:  ghPR.MergedAt,
	}, nil
}

// GetCommit returns a specific commit
func (p *GitHubProvider) GetCommit(ctx context.Context, projectID, sha string) (*Commit, error) {
	resp, err := p.doRequest(ctx, http.MethodGet, fmt.Sprintf("/repos/%s/commits/%s", projectID, sha), nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var ghCommit struct {
		SHA    string `json:"sha"`
		Commit struct {
			Message string `json:"message"`
			Author  struct {
				Name  string    `json:"name"`
				Email string    `json:"email"`
				Date  time.Time `json:"date"`
			} `json:"author"`
		} `json:"commit"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&ghCommit); err != nil {
		return nil, err
	}

	return &Commit{
		SHA:         ghCommit.SHA,
		Message:     ghCommit.Commit.Message,
		Author:      ghCommit.Commit.Author.Name,
		AuthorEmail: ghCommit.Commit.Author.Email,
		CreatedAt:   ghCommit.Commit.Author.Date,
	}, nil
}

// ListCommits returns commits for a branch
func (p *GitHubProvider) ListCommits(ctx context.Context, projectID, branch string, page, perPage int) ([]*Commit, error) {
	path := fmt.Sprintf("/repos/%s/commits?sha=%s&page=%d&per_page=%d", projectID, url.QueryEscape(branch), page, perPage)

	resp, err := p.doRequest(ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var ghCommits []struct {
		SHA    string `json:"sha"`
		Commit struct {
			Message string `json:"message"`
			Author  struct {
				Name  string    `json:"name"`
				Email string    `json:"email"`
				Date  time.Time `json:"date"`
			} `json:"author"`
		} `json:"commit"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&ghCommits); err != nil {
		return nil, err
	}

	commits := make([]*Commit, len(ghCommits))
	for i, ghc := range ghCommits {
		commits[i] = &Commit{
			SHA:         ghc.SHA,
			Message:     ghc.Commit.Message,
			Author:      ghc.Commit.Author.Name,
			AuthorEmail: ghc.Commit.Author.Email,
			CreatedAt:   ghc.Commit.Author.Date,
		}
	}

	return commits, nil
}

// RegisterWebhook registers a webhook
func (p *GitHubProvider) RegisterWebhook(ctx context.Context, projectID string, config *WebhookConfig) (string, error) {
	events := make([]string, 0, len(config.Events))
	for _, event := range config.Events {
		switch event {
		case "push":
			events = append(events, "push")
		case "merge_request":
			events = append(events, "pull_request")
		}
	}

	eventsJSON, _ := json.Marshal(events)
	body := fmt.Sprintf(`{"name":"web","active":true,"events":%s,"config":{"url":"%s","content_type":"json","secret":"%s"}}`,
		string(eventsJSON), config.URL, config.Secret)

	resp, err := p.doRequest(ctx, http.MethodPost, fmt.Sprintf("/repos/%s/hooks", projectID), strings.NewReader(body))
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	var result struct {
		ID int `json:"id"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", err
	}

	return strconv.Itoa(result.ID), nil
}

// DeleteWebhook deletes a webhook
func (p *GitHubProvider) DeleteWebhook(ctx context.Context, projectID, webhookID string) error {
	resp, err := p.doRequest(ctx, http.MethodDelete, fmt.Sprintf("/repos/%s/hooks/%s", projectID, webhookID), nil)
	if err != nil {
		return err
	}
	resp.Body.Close()
	return nil
}

// GetFileContent returns file content
func (p *GitHubProvider) GetFileContent(ctx context.Context, projectID, filePath, ref string) ([]byte, error) {
	path := fmt.Sprintf("/repos/%s/contents/%s?ref=%s", projectID, filePath, url.QueryEscape(ref))

	resp, err := p.doRequest(ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result struct {
		Content  string `json:"content"`
		Encoding string `json:"encoding"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	if result.Encoding == "base64" {
		return base64.StdEncoding.DecodeString(result.Content)
	}

	return []byte(result.Content), nil
}

// ==================== Pipeline Operations (GitHub Actions) ====================

// TriggerPipeline triggers a workflow run (GitHub Actions)
func (p *GitHubProvider) TriggerPipeline(ctx context.Context, projectID string, req *TriggerPipelineRequest) (*Pipeline, error) {
	// GitHub requires workflow_id to trigger, we'll use workflow_dispatch event
	// This is a simplified implementation - in practice you'd need to know the workflow file name
	path := fmt.Sprintf("/repos/%s/actions/workflows/ci.yml/dispatches", projectID)

	bodyData := map[string]interface{}{
		"ref": req.Ref,
	}
	if len(req.Variables) > 0 {
		bodyData["inputs"] = req.Variables
	}

	bodyBytes, err := json.Marshal(bodyData)
	if err != nil {
		return nil, err
	}

	resp, err := p.doRequest(ctx, http.MethodPost, path, strings.NewReader(string(bodyBytes)))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	// GitHub returns 204 No Content on success, fetch the latest run
	runs, err := p.ListPipelines(ctx, projectID, req.Ref, "", 1, 1)
	if err != nil {
		return nil, err
	}
	if len(runs) > 0 {
		return runs[0], nil
	}

	return &Pipeline{
		ProjectID: projectID,
		Ref:       req.Ref,
		Status:    PipelineStatusPending,
	}, nil
}

// GetPipeline returns a specific workflow run
func (p *GitHubProvider) GetPipeline(ctx context.Context, projectID string, pipelineID int) (*Pipeline, error) {
	path := fmt.Sprintf("/repos/%s/actions/runs/%d", projectID, pipelineID)

	resp, err := p.doRequest(ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	return p.parseWorkflowRun(resp.Body, projectID)
}

// ListPipelines returns workflow runs for a repository
func (p *GitHubProvider) ListPipelines(ctx context.Context, projectID string, ref, status string, page, perPage int) ([]*Pipeline, error) {
	path := fmt.Sprintf("/repos/%s/actions/runs?page=%d&per_page=%d", projectID, page, perPage)
	if ref != "" {
		path += "&branch=" + url.QueryEscape(ref)
	}
	if status != "" {
		path += "&status=" + status
	}

	resp, err := p.doRequest(ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result struct {
		WorkflowRuns []struct {
			ID           int        `json:"id"`
			RunNumber    int        `json:"run_number"`
			HeadBranch   string     `json:"head_branch"`
			HeadSHA      string     `json:"head_sha"`
			Status       string     `json:"status"`
			Conclusion   string     `json:"conclusion"`
			Event        string     `json:"event"`
			HTMLURL      string     `json:"html_url"`
			CreatedAt    time.Time  `json:"created_at"`
			UpdatedAt    time.Time  `json:"updated_at"`
			RunStartedAt *time.Time `json:"run_started_at"`
		} `json:"workflow_runs"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	pipelines := make([]*Pipeline, len(result.WorkflowRuns))
	for i, run := range result.WorkflowRuns {
		pipelines[i] = &Pipeline{
			ID:         run.ID,
			IID:        run.RunNumber,
			ProjectID:  projectID,
			Ref:        run.HeadBranch,
			SHA:        run.HeadSHA,
			Status:     p.mapGitHubStatus(run.Status, run.Conclusion),
			Source:     run.Event,
			WebURL:     run.HTMLURL,
			CreatedAt:  run.CreatedAt,
			UpdatedAt:  run.UpdatedAt,
			StartedAt:  run.RunStartedAt,
		}
	}

	return pipelines, nil
}

// CancelPipeline cancels a workflow run
func (p *GitHubProvider) CancelPipeline(ctx context.Context, projectID string, pipelineID int) (*Pipeline, error) {
	path := fmt.Sprintf("/repos/%s/actions/runs/%d/cancel", projectID, pipelineID)

	resp, err := p.doRequest(ctx, http.MethodPost, path, nil)
	if err != nil {
		return nil, err
	}
	resp.Body.Close()

	return p.GetPipeline(ctx, projectID, pipelineID)
}

// RetryPipeline re-runs a workflow
func (p *GitHubProvider) RetryPipeline(ctx context.Context, projectID string, pipelineID int) (*Pipeline, error) {
	path := fmt.Sprintf("/repos/%s/actions/runs/%d/rerun", projectID, pipelineID)

	resp, err := p.doRequest(ctx, http.MethodPost, path, nil)
	if err != nil {
		return nil, err
	}
	resp.Body.Close()

	return p.GetPipeline(ctx, projectID, pipelineID)
}

func (p *GitHubProvider) parseWorkflowRun(r io.Reader, projectID string) (*Pipeline, error) {
	var run struct {
		ID           int        `json:"id"`
		RunNumber    int        `json:"run_number"`
		HeadBranch   string     `json:"head_branch"`
		HeadSHA      string     `json:"head_sha"`
		Status       string     `json:"status"`
		Conclusion   string     `json:"conclusion"`
		Event        string     `json:"event"`
		HTMLURL      string     `json:"html_url"`
		CreatedAt    time.Time  `json:"created_at"`
		UpdatedAt    time.Time  `json:"updated_at"`
		RunStartedAt *time.Time `json:"run_started_at"`
	}

	if err := json.NewDecoder(r).Decode(&run); err != nil {
		return nil, err
	}

	return &Pipeline{
		ID:        run.ID,
		IID:       run.RunNumber,
		ProjectID: projectID,
		Ref:       run.HeadBranch,
		SHA:       run.HeadSHA,
		Status:    p.mapGitHubStatus(run.Status, run.Conclusion),
		Source:    run.Event,
		WebURL:    run.HTMLURL,
		CreatedAt: run.CreatedAt,
		UpdatedAt: run.UpdatedAt,
		StartedAt: run.RunStartedAt,
	}, nil
}

func (p *GitHubProvider) mapGitHubStatus(status, conclusion string) string {
	if status == "completed" {
		switch conclusion {
		case "success":
			return PipelineStatusSuccess
		case "failure":
			return PipelineStatusFailed
		case "cancelled":
			return PipelineStatusCanceled
		case "skipped":
			return PipelineStatusSkipped
		default:
			return PipelineStatusFailed
		}
	}
	switch status {
	case "queued":
		return PipelineStatusPending
	case "in_progress":
		return PipelineStatusRunning
	case "waiting":
		return PipelineStatusManual
	default:
		return PipelineStatusPending
	}
}

// ==================== Job Operations (GitHub Actions) ====================

// GetJob returns a specific job
func (p *GitHubProvider) GetJob(ctx context.Context, projectID string, jobID int) (*Job, error) {
	path := fmt.Sprintf("/repos/%s/actions/jobs/%d", projectID, jobID)

	resp, err := p.doRequest(ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	return p.parseGitHubJob(resp.Body)
}

// ListPipelineJobs returns jobs for a workflow run
func (p *GitHubProvider) ListPipelineJobs(ctx context.Context, projectID string, pipelineID int) ([]*Job, error) {
	path := fmt.Sprintf("/repos/%s/actions/runs/%d/jobs", projectID, pipelineID)

	resp, err := p.doRequest(ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result struct {
		Jobs []struct {
			ID          int        `json:"id"`
			Name        string     `json:"name"`
			Status      string     `json:"status"`
			Conclusion  string     `json:"conclusion"`
			HTMLURL     string     `json:"html_url"`
			RunID       int        `json:"run_id"`
			StartedAt   *time.Time `json:"started_at"`
			CompletedAt *time.Time `json:"completed_at"`
		} `json:"jobs"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	jobs := make([]*Job, len(result.Jobs))
	for i, ghJob := range result.Jobs {
		var duration float64
		if ghJob.StartedAt != nil && ghJob.CompletedAt != nil {
			duration = ghJob.CompletedAt.Sub(*ghJob.StartedAt).Seconds()
		}

		jobs[i] = &Job{
			ID:         ghJob.ID,
			Name:       ghJob.Name,
			Status:     p.mapGitHubStatus(ghJob.Status, ghJob.Conclusion),
			PipelineID: ghJob.RunID,
			WebURL:     ghJob.HTMLURL,
			CreatedAt:  time.Time{},
			StartedAt:  ghJob.StartedAt,
			FinishedAt: ghJob.CompletedAt,
			Duration:   duration,
		}
	}

	return jobs, nil
}

// RetryJob re-runs a specific job (GitHub re-runs failed jobs in a workflow)
func (p *GitHubProvider) RetryJob(ctx context.Context, projectID string, jobID int) (*Job, error) {
	path := fmt.Sprintf("/repos/%s/actions/jobs/%d/rerun", projectID, jobID)

	resp, err := p.doRequest(ctx, http.MethodPost, path, nil)
	if err != nil {
		return nil, err
	}
	resp.Body.Close()

	return p.GetJob(ctx, projectID, jobID)
}

// CancelJob cancels a workflow run (GitHub doesn't support cancelling individual jobs)
func (p *GitHubProvider) CancelJob(ctx context.Context, projectID string, jobID int) (*Job, error) {
	// GitHub doesn't support cancelling individual jobs, only entire runs
	// Return the job as-is
	return p.GetJob(ctx, projectID, jobID)
}

// GetJobTrace returns the job logs
func (p *GitHubProvider) GetJobTrace(ctx context.Context, projectID string, jobID int) (string, error) {
	path := fmt.Sprintf("/repos/%s/actions/jobs/%d/logs", projectID, jobID)

	resp, err := p.doRequest(ctx, http.MethodGet, path, nil)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	return string(data), nil
}

// GetJobArtifact downloads a specific artifact (GitHub artifacts are per-run, not per-job)
func (p *GitHubProvider) GetJobArtifact(ctx context.Context, projectID string, jobID int, artifactPath string) ([]byte, error) {
	// GitHub artifacts are associated with workflow runs, not individual jobs
	// This is a simplified implementation
	return nil, ErrNotFound
}

// DownloadJobArtifacts downloads artifacts for a workflow run
func (p *GitHubProvider) DownloadJobArtifacts(ctx context.Context, projectID string, jobID int) ([]byte, error) {
	// GitHub artifacts are associated with workflow runs, not individual jobs
	// This is a simplified implementation
	return nil, ErrNotFound
}

func (p *GitHubProvider) parseGitHubJob(r io.Reader) (*Job, error) {
	var ghJob struct {
		ID          int        `json:"id"`
		Name        string     `json:"name"`
		Status      string     `json:"status"`
		Conclusion  string     `json:"conclusion"`
		HTMLURL     string     `json:"html_url"`
		RunID       int        `json:"run_id"`
		StartedAt   *time.Time `json:"started_at"`
		CompletedAt *time.Time `json:"completed_at"`
	}

	if err := json.NewDecoder(r).Decode(&ghJob); err != nil {
		return nil, err
	}

	var duration float64
	if ghJob.StartedAt != nil && ghJob.CompletedAt != nil {
		duration = ghJob.CompletedAt.Sub(*ghJob.StartedAt).Seconds()
	}

	return &Job{
		ID:         ghJob.ID,
		Name:       ghJob.Name,
		Status:     p.mapGitHubStatus(ghJob.Status, ghJob.Conclusion),
		PipelineID: ghJob.RunID,
		WebURL:     ghJob.HTMLURL,
		CreatedAt:  time.Time{},
		StartedAt:  ghJob.StartedAt,
		FinishedAt: ghJob.CompletedAt,
		Duration:   duration,
	}, nil
}
