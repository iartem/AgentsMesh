package git

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

// GitLabProvider implements Provider interface for GitLab
type GitLabProvider struct {
	baseURL     string
	accessToken string
	httpClient  *http.Client
}

// NewGitLabProvider creates a new GitLab provider
func NewGitLabProvider(baseURL, accessToken string) (*GitLabProvider, error) {
	if baseURL == "" {
		baseURL = "https://gitlab.com"
	}
	baseURL = strings.TrimSuffix(baseURL, "/")

	return &GitLabProvider{
		baseURL:     baseURL,
		accessToken: accessToken,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}, nil
}

func (p *GitLabProvider) doRequest(ctx context.Context, method, path string, body io.Reader) (*http.Response, error) {
	reqURL := fmt.Sprintf("%s/api/v4%s", p.baseURL, path)

	req, err := http.NewRequestWithContext(ctx, method, reqURL, body)
	if err != nil {
		return nil, err
	}

	req.Header.Set("PRIVATE-TOKEN", p.accessToken)
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
	if resp.StatusCode == http.StatusTooManyRequests {
		resp.Body.Close()
		return nil, ErrRateLimited
	}

	return resp, nil
}

// GetCurrentUser returns the authenticated user
func (p *GitLabProvider) GetCurrentUser(ctx context.Context) (*User, error) {
	resp, err := p.doRequest(ctx, http.MethodGet, "/user", nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var glUser struct {
		ID        int    `json:"id"`
		Username  string `json:"username"`
		Name      string `json:"name"`
		Email     string `json:"email"`
		AvatarURL string `json:"avatar_url"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&glUser); err != nil {
		return nil, err
	}

	return &User{
		ID:        strconv.Itoa(glUser.ID),
		Username:  glUser.Username,
		Name:      glUser.Name,
		Email:     glUser.Email,
		AvatarURL: glUser.AvatarURL,
	}, nil
}

// GetProject returns project details
func (p *GitLabProvider) GetProject(ctx context.Context, projectID string) (*Project, error) {
	encodedID := url.PathEscape(projectID)
	resp, err := p.doRequest(ctx, http.MethodGet, fmt.Sprintf("/projects/%s", encodedID), nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var glProject struct {
		ID                int       `json:"id"`
		Name              string    `json:"name"`
		PathWithNamespace string    `json:"path_with_namespace"`
		Description       string    `json:"description"`
		DefaultBranch     string    `json:"default_branch"`
		WebURL            string    `json:"web_url"`
		HTTPURLToRepo     string    `json:"http_url_to_repo"`
		SSHURLToRepo      string    `json:"ssh_url_to_repo"`
		Visibility        string    `json:"visibility"`
		CreatedAt         time.Time `json:"created_at"`
		LastActivityAt    time.Time `json:"last_activity_at"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&glProject); err != nil {
		return nil, err
	}

	return &Project{
		ID:            strconv.Itoa(glProject.ID),
		Name:          glProject.Name,
		FullPath:      glProject.PathWithNamespace,
		Description:   glProject.Description,
		DefaultBranch: glProject.DefaultBranch,
		WebURL:        glProject.WebURL,
		CloneURL:      glProject.HTTPURLToRepo,
		SSHCloneURL:   glProject.SSHURLToRepo,
		Visibility:    glProject.Visibility,
		CreatedAt:     glProject.CreatedAt,
		UpdatedAt:     glProject.LastActivityAt,
	}, nil
}

// ListProjects returns user's projects
func (p *GitLabProvider) ListProjects(ctx context.Context, page, perPage int) ([]*Project, error) {
	path := fmt.Sprintf("/projects?membership=true&page=%d&per_page=%d", page, perPage)
	resp, err := p.doRequest(ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var glProjects []struct {
		ID                int       `json:"id"`
		Name              string    `json:"name"`
		PathWithNamespace string    `json:"path_with_namespace"`
		Description       string    `json:"description"`
		DefaultBranch     string    `json:"default_branch"`
		WebURL            string    `json:"web_url"`
		HTTPURLToRepo     string    `json:"http_url_to_repo"`
		SSHURLToRepo      string    `json:"ssh_url_to_repo"`
		Visibility        string    `json:"visibility"`
		CreatedAt         time.Time `json:"created_at"`
		LastActivityAt    time.Time `json:"last_activity_at"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&glProjects); err != nil {
		return nil, err
	}

	projects := make([]*Project, len(glProjects))
	for i, glp := range glProjects {
		projects[i] = &Project{
			ID:            strconv.Itoa(glp.ID),
			Name:          glp.Name,
			FullPath:      glp.PathWithNamespace,
			Description:   glp.Description,
			DefaultBranch: glp.DefaultBranch,
			WebURL:        glp.WebURL,
			CloneURL:      glp.HTTPURLToRepo,
			SSHCloneURL:   glp.SSHURLToRepo,
			Visibility:    glp.Visibility,
			CreatedAt:     glp.CreatedAt,
			UpdatedAt:     glp.LastActivityAt,
		}
	}

	return projects, nil
}

// SearchProjects searches for projects
func (p *GitLabProvider) SearchProjects(ctx context.Context, query string, page, perPage int) ([]*Project, error) {
	path := fmt.Sprintf("/projects?search=%s&page=%d&per_page=%d", url.QueryEscape(query), page, perPage)
	resp, err := p.doRequest(ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var glProjects []struct {
		ID                int       `json:"id"`
		Name              string    `json:"name"`
		PathWithNamespace string    `json:"path_with_namespace"`
		Description       string    `json:"description"`
		DefaultBranch     string    `json:"default_branch"`
		WebURL            string    `json:"web_url"`
		HTTPURLToRepo     string    `json:"http_url_to_repo"`
		SSHURLToRepo      string    `json:"ssh_url_to_repo"`
		Visibility        string    `json:"visibility"`
		CreatedAt         time.Time `json:"created_at"`
		LastActivityAt    time.Time `json:"last_activity_at"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&glProjects); err != nil {
		return nil, err
	}

	projects := make([]*Project, len(glProjects))
	for i, glp := range glProjects {
		projects[i] = &Project{
			ID:            strconv.Itoa(glp.ID),
			Name:          glp.Name,
			FullPath:      glp.PathWithNamespace,
			Description:   glp.Description,
			DefaultBranch: glp.DefaultBranch,
			WebURL:        glp.WebURL,
			CloneURL:      glp.HTTPURLToRepo,
			SSHCloneURL:   glp.SSHURLToRepo,
			Visibility:    glp.Visibility,
			CreatedAt:     glp.CreatedAt,
			UpdatedAt:     glp.LastActivityAt,
		}
	}

	return projects, nil
}

// ListBranches returns branches for a project
func (p *GitLabProvider) ListBranches(ctx context.Context, projectID string) ([]*Branch, error) {
	encodedID := url.PathEscape(projectID)
	resp, err := p.doRequest(ctx, http.MethodGet, fmt.Sprintf("/projects/%s/repository/branches", encodedID), nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var glBranches []struct {
		Name      string `json:"name"`
		Commit    struct {
			ID string `json:"id"`
		} `json:"commit"`
		Protected bool `json:"protected"`
		Default   bool `json:"default"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&glBranches); err != nil {
		return nil, err
	}

	branches := make([]*Branch, len(glBranches))
	for i, glb := range glBranches {
		branches[i] = &Branch{
			Name:      glb.Name,
			CommitSHA: glb.Commit.ID,
			Protected: glb.Protected,
			Default:   glb.Default,
		}
	}

	return branches, nil
}

// GetBranch returns a specific branch
func (p *GitLabProvider) GetBranch(ctx context.Context, projectID, branchName string) (*Branch, error) {
	encodedID := url.PathEscape(projectID)
	encodedBranch := url.PathEscape(branchName)
	resp, err := p.doRequest(ctx, http.MethodGet, fmt.Sprintf("/projects/%s/repository/branches/%s", encodedID, encodedBranch), nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var glBranch struct {
		Name      string `json:"name"`
		Commit    struct {
			ID string `json:"id"`
		} `json:"commit"`
		Protected bool `json:"protected"`
		Default   bool `json:"default"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&glBranch); err != nil {
		return nil, err
	}

	return &Branch{
		Name:      glBranch.Name,
		CommitSHA: glBranch.Commit.ID,
		Protected: glBranch.Protected,
		Default:   glBranch.Default,
	}, nil
}

// CreateBranch creates a new branch
func (p *GitLabProvider) CreateBranch(ctx context.Context, projectID, branchName, ref string) (*Branch, error) {
	encodedID := url.PathEscape(projectID)
	path := fmt.Sprintf("/projects/%s/repository/branches?branch=%s&ref=%s",
		encodedID, url.QueryEscape(branchName), url.QueryEscape(ref))

	resp, err := p.doRequest(ctx, http.MethodPost, path, nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var glBranch struct {
		Name      string `json:"name"`
		Commit    struct {
			ID string `json:"id"`
		} `json:"commit"`
		Protected bool `json:"protected"`
		Default   bool `json:"default"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&glBranch); err != nil {
		return nil, err
	}

	return &Branch{
		Name:      glBranch.Name,
		CommitSHA: glBranch.Commit.ID,
		Protected: glBranch.Protected,
		Default:   glBranch.Default,
	}, nil
}

// DeleteBranch deletes a branch
func (p *GitLabProvider) DeleteBranch(ctx context.Context, projectID, branchName string) error {
	encodedID := url.PathEscape(projectID)
	encodedBranch := url.PathEscape(branchName)
	resp, err := p.doRequest(ctx, http.MethodDelete, fmt.Sprintf("/projects/%s/repository/branches/%s", encodedID, encodedBranch), nil)
	if err != nil {
		return err
	}
	resp.Body.Close()
	return nil
}

// GetMergeRequest returns a specific merge request
func (p *GitLabProvider) GetMergeRequest(ctx context.Context, projectID string, mrIID int) (*MergeRequest, error) {
	encodedID := url.PathEscape(projectID)
	resp, err := p.doRequest(ctx, http.MethodGet, fmt.Sprintf("/projects/%s/merge_requests/%d", encodedID, mrIID), nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	return p.parseMergeRequest(resp.Body)
}

// ListMergeRequests returns merge requests for a project
func (p *GitLabProvider) ListMergeRequests(ctx context.Context, projectID string, state string, page, perPage int) ([]*MergeRequest, error) {
	encodedID := url.PathEscape(projectID)
	path := fmt.Sprintf("/projects/%s/merge_requests?page=%d&per_page=%d", encodedID, page, perPage)
	if state != "" {
		path += "&state=" + state
	}

	resp, err := p.doRequest(ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var glMRs []struct {
		ID           int       `json:"id"`
		IID          int       `json:"iid"`
		Title        string    `json:"title"`
		Description  string    `json:"description"`
		SourceBranch string    `json:"source_branch"`
		TargetBranch string    `json:"target_branch"`
		State        string    `json:"state"`
		WebURL       string    `json:"web_url"`
		Author       struct {
			ID        int    `json:"id"`
			Username  string `json:"username"`
			Name      string `json:"name"`
			AvatarURL string `json:"avatar_url"`
		} `json:"author"`
		CreatedAt time.Time  `json:"created_at"`
		UpdatedAt time.Time  `json:"updated_at"`
		MergedAt  *time.Time `json:"merged_at"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&glMRs); err != nil {
		return nil, err
	}

	mrs := make([]*MergeRequest, len(glMRs))
	for i, glMR := range glMRs {
		mrs[i] = &MergeRequest{
			ID:           glMR.ID,
			IID:          glMR.IID,
			Title:        glMR.Title,
			Description:  glMR.Description,
			SourceBranch: glMR.SourceBranch,
			TargetBranch: glMR.TargetBranch,
			State:        glMR.State,
			WebURL:       glMR.WebURL,
			Author: &User{
				ID:        strconv.Itoa(glMR.Author.ID),
				Username:  glMR.Author.Username,
				Name:      glMR.Author.Name,
				AvatarURL: glMR.Author.AvatarURL,
			},
			CreatedAt: glMR.CreatedAt,
			UpdatedAt: glMR.UpdatedAt,
			MergedAt:  glMR.MergedAt,
		}
	}

	return mrs, nil
}

// ListMergeRequestsByBranch returns merge requests filtered by source branch
func (p *GitLabProvider) ListMergeRequestsByBranch(ctx context.Context, projectID, sourceBranch, state string) ([]*MergeRequest, error) {
	encodedID := url.PathEscape(projectID)
	path := fmt.Sprintf("/projects/%s/merge_requests?source_branch=%s", encodedID, url.QueryEscape(sourceBranch))
	if state != "" && state != "all" {
		path += "&state=" + state
	}

	resp, err := p.doRequest(ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var glMRs []struct {
		ID             int        `json:"id"`
		IID            int        `json:"iid"`
		Title          string     `json:"title"`
		Description    string     `json:"description"`
		SourceBranch   string     `json:"source_branch"`
		TargetBranch   string     `json:"target_branch"`
		State          string     `json:"state"`
		WebURL         string     `json:"web_url"`
		MergeCommitSHA string     `json:"merge_commit_sha"`
		Pipeline       *struct {
			ID     int    `json:"id"`
			Status string `json:"status"`
			WebURL string `json:"web_url"`
		} `json:"pipeline"`
		Author struct {
			ID        int    `json:"id"`
			Username  string `json:"username"`
			Name      string `json:"name"`
			AvatarURL string `json:"avatar_url"`
		} `json:"author"`
		CreatedAt time.Time  `json:"created_at"`
		UpdatedAt time.Time  `json:"updated_at"`
		MergedAt  *time.Time `json:"merged_at"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&glMRs); err != nil {
		return nil, err
	}

	mrs := make([]*MergeRequest, len(glMRs))
	for i, glMR := range glMRs {
		mr := &MergeRequest{
			ID:             glMR.ID,
			IID:            glMR.IID,
			Title:          glMR.Title,
			Description:    glMR.Description,
			SourceBranch:   glMR.SourceBranch,
			TargetBranch:   glMR.TargetBranch,
			State:          glMR.State,
			WebURL:         glMR.WebURL,
			MergeCommitSHA: glMR.MergeCommitSHA,
			Author: &User{
				ID:        strconv.Itoa(glMR.Author.ID),
				Username:  glMR.Author.Username,
				Name:      glMR.Author.Name,
				AvatarURL: glMR.Author.AvatarURL,
			},
			CreatedAt: glMR.CreatedAt,
			UpdatedAt: glMR.UpdatedAt,
			MergedAt:  glMR.MergedAt,
		}
		if glMR.Pipeline != nil {
			mr.PipelineID = glMR.Pipeline.ID
			mr.PipelineStatus = glMR.Pipeline.Status
			mr.PipelineURL = glMR.Pipeline.WebURL
		}
		mrs[i] = mr
	}

	return mrs, nil
}

// CreateMergeRequest creates a new merge request
func (p *GitLabProvider) CreateMergeRequest(ctx context.Context, req *CreateMRRequest) (*MergeRequest, error) {
	encodedID := url.PathEscape(req.ProjectID)
	path := fmt.Sprintf("/projects/%s/merge_requests", encodedID)

	body := fmt.Sprintf(`{"source_branch":"%s","target_branch":"%s","title":"%s","description":"%s"}`,
		req.SourceBranch, req.TargetBranch, req.Title, req.Description)

	resp, err := p.doRequest(ctx, http.MethodPost, path, strings.NewReader(body))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	return p.parseMergeRequest(resp.Body)
}

// UpdateMergeRequest updates a merge request
func (p *GitLabProvider) UpdateMergeRequest(ctx context.Context, projectID string, mrIID int, title, description string) (*MergeRequest, error) {
	encodedID := url.PathEscape(projectID)
	path := fmt.Sprintf("/projects/%s/merge_requests/%d", encodedID, mrIID)

	body := fmt.Sprintf(`{"title":"%s","description":"%s"}`, title, description)

	resp, err := p.doRequest(ctx, http.MethodPut, path, strings.NewReader(body))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	return p.parseMergeRequest(resp.Body)
}

// MergeMergeRequest merges a merge request
func (p *GitLabProvider) MergeMergeRequest(ctx context.Context, projectID string, mrIID int) (*MergeRequest, error) {
	encodedID := url.PathEscape(projectID)
	path := fmt.Sprintf("/projects/%s/merge_requests/%d/merge", encodedID, mrIID)

	resp, err := p.doRequest(ctx, http.MethodPut, path, nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	return p.parseMergeRequest(resp.Body)
}

// CloseMergeRequest closes a merge request
func (p *GitLabProvider) CloseMergeRequest(ctx context.Context, projectID string, mrIID int) (*MergeRequest, error) {
	encodedID := url.PathEscape(projectID)
	path := fmt.Sprintf("/projects/%s/merge_requests/%d", encodedID, mrIID)

	body := `{"state_event":"close"}`

	resp, err := p.doRequest(ctx, http.MethodPut, path, strings.NewReader(body))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	return p.parseMergeRequest(resp.Body)
}

func (p *GitLabProvider) parseMergeRequest(r io.Reader) (*MergeRequest, error) {
	var glMR struct {
		ID           int       `json:"id"`
		IID          int       `json:"iid"`
		Title        string    `json:"title"`
		Description  string    `json:"description"`
		SourceBranch string    `json:"source_branch"`
		TargetBranch string    `json:"target_branch"`
		State        string    `json:"state"`
		WebURL       string    `json:"web_url"`
		Author       struct {
			ID        int    `json:"id"`
			Username  string `json:"username"`
			Name      string `json:"name"`
			AvatarURL string `json:"avatar_url"`
		} `json:"author"`
		CreatedAt time.Time  `json:"created_at"`
		UpdatedAt time.Time  `json:"updated_at"`
		MergedAt  *time.Time `json:"merged_at"`
	}

	if err := json.NewDecoder(r).Decode(&glMR); err != nil {
		return nil, err
	}

	return &MergeRequest{
		ID:           glMR.ID,
		IID:          glMR.IID,
		Title:        glMR.Title,
		Description:  glMR.Description,
		SourceBranch: glMR.SourceBranch,
		TargetBranch: glMR.TargetBranch,
		State:        glMR.State,
		WebURL:       glMR.WebURL,
		Author: &User{
			ID:        strconv.Itoa(glMR.Author.ID),
			Username:  glMR.Author.Username,
			Name:      glMR.Author.Name,
			AvatarURL: glMR.Author.AvatarURL,
		},
		CreatedAt: glMR.CreatedAt,
		UpdatedAt: glMR.UpdatedAt,
		MergedAt:  glMR.MergedAt,
	}, nil
}

// GetCommit returns a specific commit
func (p *GitLabProvider) GetCommit(ctx context.Context, projectID, sha string) (*Commit, error) {
	encodedID := url.PathEscape(projectID)
	resp, err := p.doRequest(ctx, http.MethodGet, fmt.Sprintf("/projects/%s/repository/commits/%s", encodedID, sha), nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var glCommit struct {
		ID             string    `json:"id"`
		Message        string    `json:"message"`
		AuthorName     string    `json:"author_name"`
		AuthorEmail    string    `json:"author_email"`
		CreatedAt      time.Time `json:"created_at"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&glCommit); err != nil {
		return nil, err
	}

	return &Commit{
		SHA:         glCommit.ID,
		Message:     glCommit.Message,
		Author:      glCommit.AuthorName,
		AuthorEmail: glCommit.AuthorEmail,
		CreatedAt:   glCommit.CreatedAt,
	}, nil
}

// ListCommits returns commits for a branch
func (p *GitLabProvider) ListCommits(ctx context.Context, projectID, branch string, page, perPage int) ([]*Commit, error) {
	encodedID := url.PathEscape(projectID)
	path := fmt.Sprintf("/projects/%s/repository/commits?ref_name=%s&page=%d&per_page=%d",
		encodedID, url.QueryEscape(branch), page, perPage)

	resp, err := p.doRequest(ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var glCommits []struct {
		ID             string    `json:"id"`
		Message        string    `json:"message"`
		AuthorName     string    `json:"author_name"`
		AuthorEmail    string    `json:"author_email"`
		CreatedAt      time.Time `json:"created_at"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&glCommits); err != nil {
		return nil, err
	}

	commits := make([]*Commit, len(glCommits))
	for i, glc := range glCommits {
		commits[i] = &Commit{
			SHA:         glc.ID,
			Message:     glc.Message,
			Author:      glc.AuthorName,
			AuthorEmail: glc.AuthorEmail,
			CreatedAt:   glc.CreatedAt,
		}
	}

	return commits, nil
}

// RegisterWebhook registers a webhook
func (p *GitLabProvider) RegisterWebhook(ctx context.Context, projectID string, config *WebhookConfig) (string, error) {
	encodedID := url.PathEscape(projectID)
	path := fmt.Sprintf("/projects/%s/hooks", encodedID)

	// Map events to GitLab format
	pushEvents := false
	mrEvents := false
	for _, event := range config.Events {
		switch event {
		case "push":
			pushEvents = true
		case "merge_request":
			mrEvents = true
		}
	}

	body := fmt.Sprintf(`{"url":"%s","token":"%s","push_events":%t,"merge_requests_events":%t}`,
		config.URL, config.Secret, pushEvents, mrEvents)

	resp, err := p.doRequest(ctx, http.MethodPost, path, strings.NewReader(body))
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
func (p *GitLabProvider) DeleteWebhook(ctx context.Context, projectID, webhookID string) error {
	encodedID := url.PathEscape(projectID)
	resp, err := p.doRequest(ctx, http.MethodDelete, fmt.Sprintf("/projects/%s/hooks/%s", encodedID, webhookID), nil)
	if err != nil {
		return err
	}
	resp.Body.Close()
	return nil
}

// GetFileContent returns file content
func (p *GitLabProvider) GetFileContent(ctx context.Context, projectID, filePath, ref string) ([]byte, error) {
	encodedID := url.PathEscape(projectID)
	encodedPath := url.PathEscape(filePath)
	path := fmt.Sprintf("/projects/%s/repository/files/%s/raw?ref=%s", encodedID, encodedPath, url.QueryEscape(ref))

	resp, err := p.doRequest(ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	return io.ReadAll(resp.Body)
}

// ==================== Pipeline Operations ====================

// TriggerPipeline triggers a new pipeline
func (p *GitLabProvider) TriggerPipeline(ctx context.Context, projectID string, req *TriggerPipelineRequest) (*Pipeline, error) {
	encodedID := url.PathEscape(projectID)
	path := fmt.Sprintf("/projects/%s/pipeline", encodedID)

	// Build request body
	bodyData := map[string]interface{}{
		"ref": req.Ref,
	}
	if len(req.Variables) > 0 {
		vars := make([]map[string]string, 0, len(req.Variables))
		for k, v := range req.Variables {
			vars = append(vars, map[string]string{"key": k, "value": v})
		}
		bodyData["variables"] = vars
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

	return p.parsePipeline(resp.Body, projectID)
}

// GetPipeline returns a specific pipeline
func (p *GitLabProvider) GetPipeline(ctx context.Context, projectID string, pipelineID int) (*Pipeline, error) {
	encodedID := url.PathEscape(projectID)
	path := fmt.Sprintf("/projects/%s/pipelines/%d", encodedID, pipelineID)

	resp, err := p.doRequest(ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	return p.parsePipeline(resp.Body, projectID)
}

// ListPipelines returns pipelines for a project
func (p *GitLabProvider) ListPipelines(ctx context.Context, projectID string, ref, status string, page, perPage int) ([]*Pipeline, error) {
	encodedID := url.PathEscape(projectID)
	path := fmt.Sprintf("/projects/%s/pipelines?page=%d&per_page=%d", encodedID, page, perPage)
	if ref != "" {
		path += "&ref=" + url.QueryEscape(ref)
	}
	if status != "" {
		path += "&status=" + status
	}

	resp, err := p.doRequest(ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var glPipelines []struct {
		ID         int        `json:"id"`
		IID        int        `json:"iid"`
		Ref        string     `json:"ref"`
		SHA        string     `json:"sha"`
		Status     string     `json:"status"`
		Source     string     `json:"source"`
		WebURL     string     `json:"web_url"`
		CreatedAt  time.Time  `json:"created_at"`
		UpdatedAt  time.Time  `json:"updated_at"`
		StartedAt  *time.Time `json:"started_at"`
		FinishedAt *time.Time `json:"finished_at"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&glPipelines); err != nil {
		return nil, err
	}

	pipelines := make([]*Pipeline, len(glPipelines))
	for i, glp := range glPipelines {
		pipelines[i] = &Pipeline{
			ID:         glp.ID,
			IID:        glp.IID,
			ProjectID:  projectID,
			Ref:        glp.Ref,
			SHA:        glp.SHA,
			Status:     glp.Status,
			Source:     glp.Source,
			WebURL:     glp.WebURL,
			CreatedAt:  glp.CreatedAt,
			UpdatedAt:  glp.UpdatedAt,
			StartedAt:  glp.StartedAt,
			FinishedAt: glp.FinishedAt,
		}
	}

	return pipelines, nil
}

// CancelPipeline cancels a pipeline
func (p *GitLabProvider) CancelPipeline(ctx context.Context, projectID string, pipelineID int) (*Pipeline, error) {
	encodedID := url.PathEscape(projectID)
	path := fmt.Sprintf("/projects/%s/pipelines/%d/cancel", encodedID, pipelineID)

	resp, err := p.doRequest(ctx, http.MethodPost, path, nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	return p.parsePipeline(resp.Body, projectID)
}

// RetryPipeline retries a pipeline
func (p *GitLabProvider) RetryPipeline(ctx context.Context, projectID string, pipelineID int) (*Pipeline, error) {
	encodedID := url.PathEscape(projectID)
	path := fmt.Sprintf("/projects/%s/pipelines/%d/retry", encodedID, pipelineID)

	resp, err := p.doRequest(ctx, http.MethodPost, path, nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	return p.parsePipeline(resp.Body, projectID)
}

func (p *GitLabProvider) parsePipeline(r io.Reader, projectID string) (*Pipeline, error) {
	var glp struct {
		ID         int        `json:"id"`
		IID        int        `json:"iid"`
		Ref        string     `json:"ref"`
		SHA        string     `json:"sha"`
		Status     string     `json:"status"`
		Source     string     `json:"source"`
		WebURL     string     `json:"web_url"`
		CreatedAt  time.Time  `json:"created_at"`
		UpdatedAt  time.Time  `json:"updated_at"`
		StartedAt  *time.Time `json:"started_at"`
		FinishedAt *time.Time `json:"finished_at"`
	}

	if err := json.NewDecoder(r).Decode(&glp); err != nil {
		return nil, err
	}

	return &Pipeline{
		ID:         glp.ID,
		IID:        glp.IID,
		ProjectID:  projectID,
		Ref:        glp.Ref,
		SHA:        glp.SHA,
		Status:     glp.Status,
		Source:     glp.Source,
		WebURL:     glp.WebURL,
		CreatedAt:  glp.CreatedAt,
		UpdatedAt:  glp.UpdatedAt,
		StartedAt:  glp.StartedAt,
		FinishedAt: glp.FinishedAt,
	}, nil
}

// ==================== Job Operations ====================

// GetJob returns a specific job
func (p *GitLabProvider) GetJob(ctx context.Context, projectID string, jobID int) (*Job, error) {
	encodedID := url.PathEscape(projectID)
	path := fmt.Sprintf("/projects/%s/jobs/%d", encodedID, jobID)

	resp, err := p.doRequest(ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	return p.parseJob(resp.Body)
}

// ListPipelineJobs returns jobs for a pipeline
func (p *GitLabProvider) ListPipelineJobs(ctx context.Context, projectID string, pipelineID int) ([]*Job, error) {
	encodedID := url.PathEscape(projectID)
	path := fmt.Sprintf("/projects/%s/pipelines/%d/jobs", encodedID, pipelineID)

	resp, err := p.doRequest(ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var glJobs []struct {
		ID           int        `json:"id"`
		Name         string     `json:"name"`
		Stage        string     `json:"stage"`
		Status       string     `json:"status"`
		Ref          string     `json:"ref"`
		WebURL       string     `json:"web_url"`
		AllowFailure bool       `json:"allow_failure"`
		Duration     float64    `json:"duration"`
		Pipeline     struct {
			ID int `json:"id"`
		} `json:"pipeline"`
		CreatedAt  time.Time  `json:"created_at"`
		StartedAt  *time.Time `json:"started_at"`
		FinishedAt *time.Time `json:"finished_at"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&glJobs); err != nil {
		return nil, err
	}

	jobs := make([]*Job, len(glJobs))
	for i, glj := range glJobs {
		jobs[i] = &Job{
			ID:           glj.ID,
			Name:         glj.Name,
			Stage:        glj.Stage,
			Status:       glj.Status,
			Ref:          glj.Ref,
			PipelineID:   glj.Pipeline.ID,
			WebURL:       glj.WebURL,
			AllowFailure: glj.AllowFailure,
			Duration:     glj.Duration,
			CreatedAt:    glj.CreatedAt,
			StartedAt:    glj.StartedAt,
			FinishedAt:   glj.FinishedAt,
		}
	}

	return jobs, nil
}

// RetryJob retries a job
func (p *GitLabProvider) RetryJob(ctx context.Context, projectID string, jobID int) (*Job, error) {
	encodedID := url.PathEscape(projectID)
	path := fmt.Sprintf("/projects/%s/jobs/%d/retry", encodedID, jobID)

	resp, err := p.doRequest(ctx, http.MethodPost, path, nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	return p.parseJob(resp.Body)
}

// CancelJob cancels a job
func (p *GitLabProvider) CancelJob(ctx context.Context, projectID string, jobID int) (*Job, error) {
	encodedID := url.PathEscape(projectID)
	path := fmt.Sprintf("/projects/%s/jobs/%d/cancel", encodedID, jobID)

	resp, err := p.doRequest(ctx, http.MethodPost, path, nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	return p.parseJob(resp.Body)
}

// GetJobTrace returns the job log (trace)
func (p *GitLabProvider) GetJobTrace(ctx context.Context, projectID string, jobID int) (string, error) {
	encodedID := url.PathEscape(projectID)
	path := fmt.Sprintf("/projects/%s/jobs/%d/trace", encodedID, jobID)

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

// GetJobArtifact downloads a specific artifact file from a job
func (p *GitLabProvider) GetJobArtifact(ctx context.Context, projectID string, jobID int, artifactPath string) ([]byte, error) {
	encodedID := url.PathEscape(projectID)
	encodedPath := url.PathEscape(artifactPath)
	path := fmt.Sprintf("/projects/%s/jobs/%d/artifacts/%s", encodedID, jobID, encodedPath)

	resp, err := p.doRequest(ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	return io.ReadAll(resp.Body)
}

// DownloadJobArtifacts downloads the complete artifacts archive from a job
func (p *GitLabProvider) DownloadJobArtifacts(ctx context.Context, projectID string, jobID int) ([]byte, error) {
	encodedID := url.PathEscape(projectID)
	path := fmt.Sprintf("/projects/%s/jobs/%d/artifacts", encodedID, jobID)

	resp, err := p.doRequest(ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	return io.ReadAll(resp.Body)
}

func (p *GitLabProvider) parseJob(r io.Reader) (*Job, error) {
	var glj struct {
		ID           int        `json:"id"`
		Name         string     `json:"name"`
		Stage        string     `json:"stage"`
		Status       string     `json:"status"`
		Ref          string     `json:"ref"`
		WebURL       string     `json:"web_url"`
		AllowFailure bool       `json:"allow_failure"`
		Duration     float64    `json:"duration"`
		Pipeline     struct {
			ID int `json:"id"`
		} `json:"pipeline"`
		CreatedAt  time.Time  `json:"created_at"`
		StartedAt  *time.Time `json:"started_at"`
		FinishedAt *time.Time `json:"finished_at"`
	}

	if err := json.NewDecoder(r).Decode(&glj); err != nil {
		return nil, err
	}

	return &Job{
		ID:           glj.ID,
		Name:         glj.Name,
		Stage:        glj.Stage,
		Status:       glj.Status,
		Ref:          glj.Ref,
		PipelineID:   glj.Pipeline.ID,
		WebURL:       glj.WebURL,
		AllowFailure: glj.AllowFailure,
		Duration:     glj.Duration,
		CreatedAt:    glj.CreatedAt,
		StartedAt:    glj.StartedAt,
		FinishedAt:   glj.FinishedAt,
	}, nil
}
