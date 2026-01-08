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

// GiteeProvider implements Provider interface for Gitee
type GiteeProvider struct {
	baseURL     string
	accessToken string
	httpClient  *http.Client
}

// NewGiteeProvider creates a new Gitee provider
func NewGiteeProvider(baseURL, accessToken string) (*GiteeProvider, error) {
	if baseURL == "" {
		baseURL = "https://gitee.com/api/v5"
	}
	baseURL = strings.TrimSuffix(baseURL, "/")

	return &GiteeProvider{
		baseURL:     baseURL,
		accessToken: accessToken,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}, nil
}

func (p *GiteeProvider) doRequest(ctx context.Context, method, path string, body io.Reader) (*http.Response, error) {
	reqURL := p.baseURL + path

	// Add access_token to URL for Gitee
	if strings.Contains(reqURL, "?") {
		reqURL += "&access_token=" + p.accessToken
	} else {
		reqURL += "?access_token=" + p.accessToken
	}

	req, err := http.NewRequestWithContext(ctx, method, reqURL, body)
	if err != nil {
		return nil, err
	}

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
func (p *GiteeProvider) GetCurrentUser(ctx context.Context) (*User, error) {
	resp, err := p.doRequest(ctx, http.MethodGet, "/user", nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var gtUser struct {
		ID        int    `json:"id"`
		Login     string `json:"login"`
		Name      string `json:"name"`
		Email     string `json:"email"`
		AvatarURL string `json:"avatar_url"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&gtUser); err != nil {
		return nil, err
	}

	return &User{
		ID:        strconv.Itoa(gtUser.ID),
		Username:  gtUser.Login,
		Name:      gtUser.Name,
		Email:     gtUser.Email,
		AvatarURL: gtUser.AvatarURL,
	}, nil
}

// GetProject returns repository details
func (p *GiteeProvider) GetProject(ctx context.Context, projectID string) (*Project, error) {
	// projectID format: owner/repo
	resp, err := p.doRequest(ctx, http.MethodGet, fmt.Sprintf("/repos/%s", projectID), nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var gtRepo struct {
		ID            int       `json:"id"`
		Name          string    `json:"name"`
		FullName      string    `json:"full_name"`
		Description   string    `json:"description"`
		DefaultBranch string    `json:"default_branch"`
		HTMLURL       string    `json:"html_url"`
		CloneURL      string    `json:"clone_url,omitempty"`
		SSHURL        string    `json:"ssh_url"`
		Public        bool      `json:"public"`
		CreatedAt     time.Time `json:"created_at"`
		UpdatedAt     time.Time `json:"updated_at"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&gtRepo); err != nil {
		return nil, err
	}

	visibility := "private"
	if gtRepo.Public {
		visibility = "public"
	}

	return &Project{
		ID:            strconv.Itoa(gtRepo.ID),
		Name:          gtRepo.Name,
		FullPath:      gtRepo.FullName,
		Description:   gtRepo.Description,
		DefaultBranch: gtRepo.DefaultBranch,
		WebURL:        gtRepo.HTMLURL,
		CloneURL:      gtRepo.CloneURL,
		SSHCloneURL:   gtRepo.SSHURL,
		Visibility:    visibility,
		CreatedAt:     gtRepo.CreatedAt,
		UpdatedAt:     gtRepo.UpdatedAt,
	}, nil
}

// ListProjects returns user's repositories
func (p *GiteeProvider) ListProjects(ctx context.Context, page, perPage int) ([]*Project, error) {
	path := fmt.Sprintf("/user/repos?page=%d&per_page=%d&sort=updated", page, perPage)
	resp, err := p.doRequest(ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var gtRepos []struct {
		ID            int       `json:"id"`
		Name          string    `json:"name"`
		FullName      string    `json:"full_name"`
		Description   string    `json:"description"`
		DefaultBranch string    `json:"default_branch"`
		HTMLURL       string    `json:"html_url"`
		CloneURL      string    `json:"clone_url,omitempty"`
		SSHURL        string    `json:"ssh_url"`
		Public        bool      `json:"public"`
		CreatedAt     time.Time `json:"created_at"`
		UpdatedAt     time.Time `json:"updated_at"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&gtRepos); err != nil {
		return nil, err
	}

	projects := make([]*Project, len(gtRepos))
	for i, gtr := range gtRepos {
		visibility := "private"
		if gtr.Public {
			visibility = "public"
		}
		projects[i] = &Project{
			ID:            strconv.Itoa(gtr.ID),
			Name:          gtr.Name,
			FullPath:      gtr.FullName,
			Description:   gtr.Description,
			DefaultBranch: gtr.DefaultBranch,
			WebURL:        gtr.HTMLURL,
			CloneURL:      gtr.CloneURL,
			SSHCloneURL:   gtr.SSHURL,
			Visibility:    visibility,
			CreatedAt:     gtr.CreatedAt,
			UpdatedAt:     gtr.UpdatedAt,
		}
	}

	return projects, nil
}

// SearchProjects searches for repositories
func (p *GiteeProvider) SearchProjects(ctx context.Context, query string, page, perPage int) ([]*Project, error) {
	path := fmt.Sprintf("/search/repositories?q=%s&page=%d&per_page=%d", url.QueryEscape(query), page, perPage)
	resp, err := p.doRequest(ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var gtRepos []struct {
		ID            int       `json:"id"`
		Name          string    `json:"name"`
		FullName      string    `json:"full_name"`
		Description   string    `json:"description"`
		DefaultBranch string    `json:"default_branch"`
		HTMLURL       string    `json:"html_url"`
		CloneURL      string    `json:"clone_url,omitempty"`
		SSHURL        string    `json:"ssh_url"`
		Public        bool      `json:"public"`
		CreatedAt     time.Time `json:"created_at"`
		UpdatedAt     time.Time `json:"updated_at"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&gtRepos); err != nil {
		return nil, err
	}

	projects := make([]*Project, len(gtRepos))
	for i, gtr := range gtRepos {
		visibility := "private"
		if gtr.Public {
			visibility = "public"
		}
		projects[i] = &Project{
			ID:            strconv.Itoa(gtr.ID),
			Name:          gtr.Name,
			FullPath:      gtr.FullName,
			Description:   gtr.Description,
			DefaultBranch: gtr.DefaultBranch,
			WebURL:        gtr.HTMLURL,
			CloneURL:      gtr.CloneURL,
			SSHCloneURL:   gtr.SSHURL,
			Visibility:    visibility,
			CreatedAt:     gtr.CreatedAt,
			UpdatedAt:     gtr.UpdatedAt,
		}
	}

	return projects, nil
}

// ListBranches returns branches for a repository
func (p *GiteeProvider) ListBranches(ctx context.Context, projectID string) ([]*Branch, error) {
	resp, err := p.doRequest(ctx, http.MethodGet, fmt.Sprintf("/repos/%s/branches", projectID), nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var gtBranches []struct {
		Name   string `json:"name"`
		Commit struct {
			SHA string `json:"sha"`
		} `json:"commit"`
		Protected bool `json:"protected"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&gtBranches); err != nil {
		return nil, err
	}

	project, _ := p.GetProject(ctx, projectID)
	defaultBranch := ""
	if project != nil {
		defaultBranch = project.DefaultBranch
	}

	branches := make([]*Branch, len(gtBranches))
	for i, gtb := range gtBranches {
		branches[i] = &Branch{
			Name:      gtb.Name,
			CommitSHA: gtb.Commit.SHA,
			Protected: gtb.Protected,
			Default:   gtb.Name == defaultBranch,
		}
	}

	return branches, nil
}

// GetBranch returns a specific branch
func (p *GiteeProvider) GetBranch(ctx context.Context, projectID, branchName string) (*Branch, error) {
	resp, err := p.doRequest(ctx, http.MethodGet, fmt.Sprintf("/repos/%s/branches/%s", projectID, url.PathEscape(branchName)), nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var gtBranch struct {
		Name   string `json:"name"`
		Commit struct {
			SHA string `json:"sha"`
		} `json:"commit"`
		Protected bool `json:"protected"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&gtBranch); err != nil {
		return nil, err
	}

	project, _ := p.GetProject(ctx, projectID)
	isDefault := project != nil && gtBranch.Name == project.DefaultBranch

	return &Branch{
		Name:      gtBranch.Name,
		CommitSHA: gtBranch.Commit.SHA,
		Protected: gtBranch.Protected,
		Default:   isDefault,
	}, nil
}

// CreateBranch creates a new branch
func (p *GiteeProvider) CreateBranch(ctx context.Context, projectID, branchName, ref string) (*Branch, error) {
	body := fmt.Sprintf(`{"refs":"%s","branch_name":"%s"}`, ref, branchName)
	resp, err := p.doRequest(ctx, http.MethodPost, fmt.Sprintf("/repos/%s/branches", projectID), strings.NewReader(body))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var gtBranch struct {
		Name   string `json:"name"`
		Commit struct {
			SHA string `json:"sha"`
		} `json:"commit"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&gtBranch); err != nil {
		return nil, err
	}

	return &Branch{
		Name:      gtBranch.Name,
		CommitSHA: gtBranch.Commit.SHA,
		Protected: false,
		Default:   false,
	}, nil
}

// DeleteBranch deletes a branch
func (p *GiteeProvider) DeleteBranch(ctx context.Context, projectID, branchName string) error {
	resp, err := p.doRequest(ctx, http.MethodDelete, fmt.Sprintf("/repos/%s/branches/%s", projectID, url.PathEscape(branchName)), nil)
	if err != nil {
		return err
	}
	resp.Body.Close()
	return nil
}

// GetMergeRequest returns a specific pull request
func (p *GiteeProvider) GetMergeRequest(ctx context.Context, projectID string, mrIID int) (*MergeRequest, error) {
	resp, err := p.doRequest(ctx, http.MethodGet, fmt.Sprintf("/repos/%s/pulls/%d", projectID, mrIID), nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	return p.parsePullRequest(resp.Body)
}

// ListMergeRequests returns pull requests for a repository
func (p *GiteeProvider) ListMergeRequests(ctx context.Context, projectID string, state string, page, perPage int) ([]*MergeRequest, error) {
	gtState := "all"
	if state == "opened" {
		gtState = "open"
	} else if state == "merged" {
		gtState = "merged"
	} else if state == "closed" {
		gtState = "closed"
	}

	path := fmt.Sprintf("/repos/%s/pulls?state=%s&page=%d&per_page=%d", projectID, gtState, page, perPage)
	resp, err := p.doRequest(ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var gtPRs []struct {
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
			Name      string `json:"name"`
			AvatarURL string `json:"avatar_url"`
		} `json:"user"`
		CreatedAt time.Time  `json:"created_at"`
		UpdatedAt time.Time  `json:"updated_at"`
		MergedAt  *time.Time `json:"merged_at"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&gtPRs); err != nil {
		return nil, err
	}

	mrs := make([]*MergeRequest, len(gtPRs))
	for i, gtPR := range gtPRs {
		mrs[i] = &MergeRequest{
			ID:           gtPR.ID,
			IID:          gtPR.Number,
			Title:        gtPR.Title,
			Description:  gtPR.Body,
			SourceBranch: gtPR.Head.Ref,
			TargetBranch: gtPR.Base.Ref,
			State:        gtPR.State,
			WebURL:       gtPR.HTMLURL,
			Author: &User{
				ID:        strconv.Itoa(gtPR.User.ID),
				Username:  gtPR.User.Login,
				Name:      gtPR.User.Name,
				AvatarURL: gtPR.User.AvatarURL,
			},
			CreatedAt: gtPR.CreatedAt,
			UpdatedAt: gtPR.UpdatedAt,
			MergedAt:  gtPR.MergedAt,
		}
	}

	return mrs, nil
}

// ListMergeRequestsByBranch returns pull requests filtered by source branch
func (p *GiteeProvider) ListMergeRequestsByBranch(ctx context.Context, projectID, sourceBranch, state string) ([]*MergeRequest, error) {
	gtState := "all"
	if state == "opened" {
		gtState = "open"
	} else if state == "merged" {
		gtState = "merged"
	} else if state == "closed" {
		gtState = "closed"
	}

	// Gitee supports head parameter for filtering by source branch
	path := fmt.Sprintf("/repos/%s/pulls?state=%s&head=%s", projectID, gtState, url.QueryEscape(sourceBranch))
	resp, err := p.doRequest(ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var gtPRs []struct {
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
			Name      string `json:"name"`
			AvatarURL string `json:"avatar_url"`
		} `json:"user"`
		CreatedAt time.Time `json:"created_at"`
		UpdatedAt time.Time `json:"updated_at"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&gtPRs); err != nil {
		return nil, err
	}

	mrs := make([]*MergeRequest, len(gtPRs))
	for i, gtPR := range gtPRs {
		mrs[i] = &MergeRequest{
			ID:             gtPR.ID,
			IID:            gtPR.Number,
			Title:          gtPR.Title,
			Description:    gtPR.Body,
			SourceBranch:   gtPR.Head.Ref,
			TargetBranch:   gtPR.Base.Ref,
			State:          gtPR.State,
			WebURL:         gtPR.HTMLURL,
			MergeCommitSHA: gtPR.MergeCommitSHA,
			MergedAt:       gtPR.MergedAt,
			Author: &User{
				ID:        strconv.Itoa(gtPR.User.ID),
				Username:  gtPR.User.Login,
				Name:      gtPR.User.Name,
				AvatarURL: gtPR.User.AvatarURL,
			},
			CreatedAt: gtPR.CreatedAt,
			UpdatedAt: gtPR.UpdatedAt,
		}
	}

	return mrs, nil
}

// CreateMergeRequest creates a new pull request
func (p *GiteeProvider) CreateMergeRequest(ctx context.Context, req *CreateMRRequest) (*MergeRequest, error) {
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
func (p *GiteeProvider) UpdateMergeRequest(ctx context.Context, projectID string, mrIID int, title, description string) (*MergeRequest, error) {
	body := fmt.Sprintf(`{"title":"%s","body":"%s"}`, title, description)

	resp, err := p.doRequest(ctx, http.MethodPatch, fmt.Sprintf("/repos/%s/pulls/%d", projectID, mrIID), strings.NewReader(body))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	return p.parsePullRequest(resp.Body)
}

// MergeMergeRequest merges a pull request
func (p *GiteeProvider) MergeMergeRequest(ctx context.Context, projectID string, mrIID int) (*MergeRequest, error) {
	resp, err := p.doRequest(ctx, http.MethodPut, fmt.Sprintf("/repos/%s/pulls/%d/merge", projectID, mrIID), nil)
	if err != nil {
		return nil, err
	}
	resp.Body.Close()

	return p.GetMergeRequest(ctx, projectID, mrIID)
}

// CloseMergeRequest closes a pull request
func (p *GiteeProvider) CloseMergeRequest(ctx context.Context, projectID string, mrIID int) (*MergeRequest, error) {
	body := `{"state":"closed"}`

	resp, err := p.doRequest(ctx, http.MethodPatch, fmt.Sprintf("/repos/%s/pulls/%d", projectID, mrIID), strings.NewReader(body))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	return p.parsePullRequest(resp.Body)
}

func (p *GiteeProvider) parsePullRequest(r io.Reader) (*MergeRequest, error) {
	var gtPR struct {
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
			Name      string `json:"name"`
			AvatarURL string `json:"avatar_url"`
		} `json:"user"`
		CreatedAt time.Time  `json:"created_at"`
		UpdatedAt time.Time  `json:"updated_at"`
		MergedAt  *time.Time `json:"merged_at"`
	}

	if err := json.NewDecoder(r).Decode(&gtPR); err != nil {
		return nil, err
	}

	return &MergeRequest{
		ID:           gtPR.ID,
		IID:          gtPR.Number,
		Title:        gtPR.Title,
		Description:  gtPR.Body,
		SourceBranch: gtPR.Head.Ref,
		TargetBranch: gtPR.Base.Ref,
		State:        gtPR.State,
		WebURL:       gtPR.HTMLURL,
		Author: &User{
			ID:        strconv.Itoa(gtPR.User.ID),
			Username:  gtPR.User.Login,
			Name:      gtPR.User.Name,
			AvatarURL: gtPR.User.AvatarURL,
		},
		CreatedAt: gtPR.CreatedAt,
		UpdatedAt: gtPR.UpdatedAt,
		MergedAt:  gtPR.MergedAt,
	}, nil
}

// GetCommit returns a specific commit
func (p *GiteeProvider) GetCommit(ctx context.Context, projectID, sha string) (*Commit, error) {
	resp, err := p.doRequest(ctx, http.MethodGet, fmt.Sprintf("/repos/%s/commits/%s", projectID, sha), nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var gtCommit struct {
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

	if err := json.NewDecoder(resp.Body).Decode(&gtCommit); err != nil {
		return nil, err
	}

	return &Commit{
		SHA:         gtCommit.SHA,
		Message:     gtCommit.Commit.Message,
		Author:      gtCommit.Commit.Author.Name,
		AuthorEmail: gtCommit.Commit.Author.Email,
		CreatedAt:   gtCommit.Commit.Author.Date,
	}, nil
}

// ListCommits returns commits for a branch
func (p *GiteeProvider) ListCommits(ctx context.Context, projectID, branch string, page, perPage int) ([]*Commit, error) {
	path := fmt.Sprintf("/repos/%s/commits?sha=%s&page=%d&per_page=%d", projectID, url.QueryEscape(branch), page, perPage)

	resp, err := p.doRequest(ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var gtCommits []struct {
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

	if err := json.NewDecoder(resp.Body).Decode(&gtCommits); err != nil {
		return nil, err
	}

	commits := make([]*Commit, len(gtCommits))
	for i, gtc := range gtCommits {
		commits[i] = &Commit{
			SHA:         gtc.SHA,
			Message:     gtc.Commit.Message,
			Author:      gtc.Commit.Author.Name,
			AuthorEmail: gtc.Commit.Author.Email,
			CreatedAt:   gtc.Commit.Author.Date,
		}
	}

	return commits, nil
}

// RegisterWebhook registers a webhook
func (p *GiteeProvider) RegisterWebhook(ctx context.Context, projectID string, config *WebhookConfig) (string, error) {
	pushEvents := false
	prEvents := false
	for _, event := range config.Events {
		switch event {
		case "push":
			pushEvents = true
		case "merge_request":
			prEvents = true
		}
	}

	body := fmt.Sprintf(`{"url":"%s","password":"%s","push_events":%t,"pull_request_events":%t}`,
		config.URL, config.Secret, pushEvents, prEvents)

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
func (p *GiteeProvider) DeleteWebhook(ctx context.Context, projectID, webhookID string) error {
	resp, err := p.doRequest(ctx, http.MethodDelete, fmt.Sprintf("/repos/%s/hooks/%s", projectID, webhookID), nil)
	if err != nil {
		return err
	}
	resp.Body.Close()
	return nil
}

// GetFileContent returns file content
func (p *GiteeProvider) GetFileContent(ctx context.Context, projectID, filePath, ref string) ([]byte, error) {
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

// ==================== Pipeline Operations (Gitee Go) ====================
// Note: Gitee has limited pipeline API support compared to GitLab/GitHub.
// These are placeholder implementations that return not-supported errors.

var ErrGiteePipelineNotSupported = fmt.Errorf("gitee pipeline API not fully supported")

// TriggerPipeline is not fully supported by Gitee API
func (p *GiteeProvider) TriggerPipeline(ctx context.Context, projectID string, req *TriggerPipelineRequest) (*Pipeline, error) {
	return nil, ErrGiteePipelineNotSupported
}

// GetPipeline is not fully supported by Gitee API
func (p *GiteeProvider) GetPipeline(ctx context.Context, projectID string, pipelineID int) (*Pipeline, error) {
	return nil, ErrGiteePipelineNotSupported
}

// ListPipelines is not fully supported by Gitee API
func (p *GiteeProvider) ListPipelines(ctx context.Context, projectID string, ref, status string, page, perPage int) ([]*Pipeline, error) {
	return nil, ErrGiteePipelineNotSupported
}

// CancelPipeline is not fully supported by Gitee API
func (p *GiteeProvider) CancelPipeline(ctx context.Context, projectID string, pipelineID int) (*Pipeline, error) {
	return nil, ErrGiteePipelineNotSupported
}

// RetryPipeline is not fully supported by Gitee API
func (p *GiteeProvider) RetryPipeline(ctx context.Context, projectID string, pipelineID int) (*Pipeline, error) {
	return nil, ErrGiteePipelineNotSupported
}

// ==================== Job Operations (Gitee Go) ====================

// GetJob is not fully supported by Gitee API
func (p *GiteeProvider) GetJob(ctx context.Context, projectID string, jobID int) (*Job, error) {
	return nil, ErrGiteePipelineNotSupported
}

// ListPipelineJobs is not fully supported by Gitee API
func (p *GiteeProvider) ListPipelineJobs(ctx context.Context, projectID string, pipelineID int) ([]*Job, error) {
	return nil, ErrGiteePipelineNotSupported
}

// RetryJob is not fully supported by Gitee API
func (p *GiteeProvider) RetryJob(ctx context.Context, projectID string, jobID int) (*Job, error) {
	return nil, ErrGiteePipelineNotSupported
}

// CancelJob is not fully supported by Gitee API
func (p *GiteeProvider) CancelJob(ctx context.Context, projectID string, jobID int) (*Job, error) {
	return nil, ErrGiteePipelineNotSupported
}

// GetJobTrace is not fully supported by Gitee API
func (p *GiteeProvider) GetJobTrace(ctx context.Context, projectID string, jobID int) (string, error) {
	return "", ErrGiteePipelineNotSupported
}

// GetJobArtifact is not fully supported by Gitee API
func (p *GiteeProvider) GetJobArtifact(ctx context.Context, projectID string, jobID int, artifactPath string) ([]byte, error) {
	return nil, ErrGiteePipelineNotSupported
}

// DownloadJobArtifacts is not fully supported by Gitee API
func (p *GiteeProvider) DownloadJobArtifacts(ctx context.Context, projectID string, jobID int) ([]byte, error) {
	return nil, ErrGiteePipelineNotSupported
}
