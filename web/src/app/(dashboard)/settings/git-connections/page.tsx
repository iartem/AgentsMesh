"use client";

import { useState, useEffect, useCallback } from "react";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import {
  gitConnectionApi,
  GitConnectionData,
  RemoteRepositoryData,
} from "@/lib/api/client";
import { useTranslations } from "@/lib/i18n/client";

// Provider icons
const ProviderIcon = ({ provider }: { provider: string }) => {
  switch (provider) {
    case "github":
      return (
        <svg className="w-5 h-5" fill="currentColor" viewBox="0 0 24 24">
          <path d="M12 0c-6.626 0-12 5.373-12 12 0 5.302 3.438 9.8 8.207 11.387.599.111.793-.261.793-.577v-2.234c-3.338.726-4.033-1.416-4.033-1.416-.546-1.387-1.333-1.756-1.333-1.756-1.089-.745.083-.729.083-.729 1.205.084 1.839 1.237 1.839 1.237 1.07 1.834 2.807 1.304 3.492.997.107-.775.418-1.305.762-1.604-2.665-.305-5.467-1.334-5.467-5.931 0-1.311.469-2.381 1.236-3.221-.124-.303-.535-1.524.117-3.176 0 0 1.008-.322 3.301 1.23.957-.266 1.983-.399 3.003-.404 1.02.005 2.047.138 3.006.404 2.291-1.552 3.297-1.23 3.297-1.23.653 1.653.242 2.874.118 3.176.77.84 1.235 1.911 1.235 3.221 0 4.609-2.807 5.624-5.479 5.921.43.372.823 1.102.823 2.222v3.293c0 .319.192.694.801.576 4.765-1.589 8.199-6.086 8.199-11.386 0-6.627-5.373-12-12-12z" />
        </svg>
      );
    case "gitlab":
      return (
        <svg className="w-5 h-5" fill="currentColor" viewBox="0 0 24 24">
          <path d="m23.6 9.593-.033-.086L20.3.98a.851.851 0 0 0-.336-.405.875.875 0 0 0-1.004.054.868.868 0 0 0-.29.44l-2.208 6.763H7.538L5.33 1.07a.857.857 0 0 0-.29-.441.875.875 0 0 0-1.004-.053.851.851 0 0 0-.336.404L.433 9.502l-.032.086a6.066 6.066 0 0 0 2.012 7.01l.011.008.028.02 4.97 3.722 2.458 1.86 1.496 1.131a1.008 1.008 0 0 0 1.22 0l1.496-1.131 2.458-1.86 5-3.743.012-.01a6.068 6.068 0 0 0 2.009-7.002Z" />
        </svg>
      );
    case "gitee":
      return (
        <svg className="w-5 h-5" fill="currentColor" viewBox="0 0 24 24">
          <path d="M11.984 0A12 12 0 0 0 0 12a12 12 0 0 0 12 12 12 12 0 0 0 12-12A12 12 0 0 0 12 0a12 12 0 0 0-.016 0zm6.09 5.333c.328 0 .593.266.592.593v1.482a.594.594 0 0 1-.593.592H9.777c-.982 0-1.778.796-1.778 1.778v5.63c0 .327.266.592.593.592h5.63c.982 0 1.778-.796 1.778-1.778v-.296a.593.593 0 0 0-.592-.593h-4.15a.592.592 0 0 1-.592-.592v-1.482a.593.593 0 0 1 .593-.592h6.815c.327 0 .593.265.593.592v3.408a4 4 0 0 1-4 4H5.926a.593.593 0 0 1-.593-.593V9.778a4.444 4.444 0 0 1 4.445-4.444h8.296Z" />
        </svg>
      );
    default:
      return (
        <svg className="w-5 h-5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
          <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M10 20l4-16m4 4l4 4-4 4M6 16l-4-4 4-4" />
        </svg>
      );
  }
};

export default function GitConnectionsPage() {
  const t = useTranslations();
  const [connections, setConnections] = useState<GitConnectionData[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [showAddDialog, setShowAddDialog] = useState(false);
  const [showRepoDialog, setShowRepoDialog] = useState(false);
  const [selectedConnection, setSelectedConnection] = useState<GitConnectionData | null>(null);

  const loadConnections = useCallback(async () => {
    try {
      setLoading(true);
      setError(null);
      const response = await gitConnectionApi.list();
      setConnections(response.connections || []);
    } catch (err) {
      console.error("Failed to load connections:", err);
      setError(t("settings.gitConnections.failedToLoad"));
    } finally {
      setLoading(false);
    }
  }, [t]);

  useEffect(() => {
    loadConnections();
  }, [loadConnections]);

  const handleDelete = async (connection: GitConnectionData) => {
    if (connection.type === "oauth") {
      alert(t("settings.gitConnections.oauthCannotDelete"));
      return;
    }
    if (!confirm(t("settings.gitConnections.confirmDelete", { name: connection.provider_name }))) {
      return;
    }
    try {
      await gitConnectionApi.delete(connection.id);
      await loadConnections();
    } catch (err) {
      console.error("Failed to delete connection:", err);
      setError(t("settings.gitConnections.failedToDelete"));
    }
  };

  const handleBrowseRepos = (connection: GitConnectionData) => {
    setSelectedConnection(connection);
    setShowRepoDialog(true);
  };

  return (
    <div className="p-6 max-w-4xl mx-auto">
      <div className="mb-6">
        <h1 className="text-2xl font-bold text-foreground">{t("settings.gitConnections.title")}</h1>
        <p className="text-muted-foreground">
          {t("settings.gitConnections.description")}
        </p>
      </div>

      {error && (
        <div className="mb-4 p-4 bg-destructive/10 text-destructive rounded-lg">
          {error}
        </div>
      )}

      <div className="space-y-4">
        {/* OAuth Connections Section */}
        <div className="border border-border rounded-lg p-6">
          <div className="flex items-center justify-between mb-4">
            <div>
              <h2 className="text-lg font-semibold">{t("settings.gitConnections.oauthConnections")}</h2>
              <p className="text-sm text-muted-foreground">
                {t("settings.gitConnections.oauthDescription")}
              </p>
            </div>
          </div>

          {loading ? (
            <div className="flex items-center justify-center py-8">
              <div className="animate-spin rounded-full h-8 w-8 border-b-2 border-primary"></div>
            </div>
          ) : (
            <div className="space-y-3">
              {connections
                .filter((c) => c.type === "oauth")
                .map((connection) => (
                  <ConnectionCard
                    key={connection.id}
                    connection={connection}
                    onBrowse={() => handleBrowseRepos(connection)}
                    onDelete={() => handleDelete(connection)}
                  />
                ))}
              {connections.filter((c) => c.type === "oauth").length === 0 && (
                <p className="text-sm text-muted-foreground py-4">
                  {t("settings.gitConnections.noOauthConnections")}
                </p>
              )}
            </div>
          )}
        </div>

        {/* Personal Connections Section */}
        <div className="border border-border rounded-lg p-6">
          <div className="flex items-center justify-between mb-4">
            <div>
              <h2 className="text-lg font-semibold">{t("settings.gitConnections.personalConnections")}</h2>
              <p className="text-sm text-muted-foreground">
                {t("settings.gitConnections.personalDescription")}
              </p>
            </div>
            <Button onClick={() => setShowAddDialog(true)}>
              <svg
                className="w-4 h-4 mr-2"
                fill="none"
                viewBox="0 0 24 24"
                stroke="currentColor"
              >
                <path
                  strokeLinecap="round"
                  strokeLinejoin="round"
                  strokeWidth={2}
                  d="M12 4v16m8-8H4"
                />
              </svg>
              {t("settings.gitConnections.addConnection")}
            </Button>
          </div>

          {loading ? (
            <div className="flex items-center justify-center py-8">
              <div className="animate-spin rounded-full h-8 w-8 border-b-2 border-primary"></div>
            </div>
          ) : (
            <div className="space-y-3">
              {connections
                .filter((c) => c.type === "personal")
                .map((connection) => (
                  <ConnectionCard
                    key={connection.id}
                    connection={connection}
                    onBrowse={() => handleBrowseRepos(connection)}
                    onDelete={() => handleDelete(connection)}
                  />
                ))}
              {connections.filter((c) => c.type === "personal").length === 0 && (
                <p className="text-sm text-muted-foreground py-4">
                  {t("settings.gitConnections.noPersonalConnections")}
                </p>
              )}
            </div>
          )}
        </div>
      </div>

      {/* Add Connection Dialog */}
      {showAddDialog && (
        <AddConnectionDialog
          onClose={() => setShowAddDialog(false)}
          onSuccess={() => {
            setShowAddDialog(false);
            loadConnections();
          }}
        />
      )}

      {/* Repository Browser Dialog */}
      {showRepoDialog && selectedConnection && (
        <RepositoryBrowserDialog
          connection={selectedConnection}
          onClose={() => {
            setShowRepoDialog(false);
            setSelectedConnection(null);
          }}
        />
      )}
    </div>
  );
}

// Connection Card Component
function ConnectionCard({
  connection,
  onBrowse,
  onDelete,
}: {
  connection: GitConnectionData;
  onBrowse: () => void;
  onDelete: () => void;
}) {
  const t = useTranslations();
  return (
    <div className="flex items-center justify-between p-4 bg-muted/50 rounded-lg">
      <div className="flex items-center gap-4">
        <div className="w-10 h-10 rounded-full bg-background flex items-center justify-center">
          <ProviderIcon provider={connection.provider_type} />
        </div>
        <div>
          <div className="flex items-center gap-2">
            <span className="font-medium">{connection.provider_name}</span>
            {connection.is_active ? (
              <span className="px-2 py-0.5 text-xs bg-green-500/10 text-green-600 rounded-full">
                {t("settings.gitConnections.active")}
              </span>
            ) : (
              <span className="px-2 py-0.5 text-xs bg-yellow-500/10 text-yellow-600 rounded-full">
                {t("settings.gitConnections.inactive")}
              </span>
            )}
            {connection.type === "oauth" && (
              <span className="px-2 py-0.5 text-xs bg-blue-500/10 text-blue-600 rounded-full">
                OAuth
              </span>
            )}
          </div>
          <div className="text-sm text-muted-foreground">
            {connection.username && `@${connection.username} • `}
            {connection.base_url}
          </div>
        </div>
      </div>
      <div className="flex items-center gap-2">
        <Button variant="outline" size="sm" onClick={onBrowse}>
          {t("settings.gitConnections.browseRepos")}
        </Button>
        {connection.type === "personal" && (
          <Button variant="ghost" size="sm" onClick={onDelete}>
            <svg
              className="w-4 h-4 text-destructive"
              fill="none"
              viewBox="0 0 24 24"
              stroke="currentColor"
            >
              <path
                strokeLinecap="round"
                strokeLinejoin="round"
                strokeWidth={2}
                d="M19 7l-.867 12.142A2 2 0 0116.138 21H7.862a2 2 0 01-1.995-1.858L5 7m5 4v6m4-6v6m1-10V4a1 1 0 00-1-1h-4a1 1 0 00-1 1v3M4 7h16"
              />
            </svg>
          </Button>
        )}
      </div>
    </div>
  );
}

// Add Connection Dialog
function AddConnectionDialog({
  onClose,
  onSuccess,
}: {
  onClose: () => void;
  onSuccess: () => void;
}) {
  const t = useTranslations();
  const [step, setStep] = useState<"provider" | "credentials">("provider");
  const [providerType, setProviderType] = useState<string>("");
  const [providerName, setProviderName] = useState("");
  const [baseURL, setBaseURL] = useState("");
  const [authType, setAuthType] = useState<"pat" | "ssh">("pat");
  const [accessToken, setAccessToken] = useState("");
  const [sshPrivateKey, setSSHPrivateKey] = useState("");
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const providers = [
    {
      type: "github",
      name: "GitHub",
      defaultURL: "https://github.com",
      descriptionKey: "settings.gitConnections.dialog.githubDescription",
    },
    {
      type: "gitlab",
      name: "GitLab",
      defaultURL: "https://gitlab.com",
      descriptionKey: "settings.gitConnections.dialog.gitlabDescription",
    },
    {
      type: "gitee",
      name: "Gitee",
      defaultURL: "https://gitee.com",
      descriptionKey: "settings.gitConnections.dialog.giteeDescription",
    },
  ];

  const selectProvider = (type: string) => {
    const provider = providers.find((p) => p.type === type);
    setProviderType(type);
    setProviderName(provider?.name || "");
    setBaseURL(provider?.defaultURL || "");
    setStep("credentials");
  };

  const handleSubmit = async () => {
    if (!providerType || !providerName || !baseURL) {
      setError(t("settings.gitConnections.dialog.fillAllFields"));
      return;
    }
    if (authType === "pat" && !accessToken) {
      setError(t("settings.gitConnections.dialog.enterPat"));
      return;
    }
    if (authType === "ssh" && !sshPrivateKey) {
      setError(t("settings.gitConnections.dialog.enterSshKey"));
      return;
    }

    setSaving(true);
    setError(null);

    try {
      await gitConnectionApi.create({
        provider_type: providerType,
        provider_name: providerName,
        base_url: baseURL,
        auth_type: authType,
        access_token: authType === "pat" ? accessToken : undefined,
        ssh_private_key: authType === "ssh" ? sshPrivateKey : undefined,
      });
      onSuccess();
    } catch (err) {
      console.error("Failed to create connection:", err);
      setError(t("settings.gitConnections.dialog.failedToCreate"));
    } finally {
      setSaving(false);
    }
  };

  return (
    <div className="fixed inset-0 bg-black/50 flex items-center justify-center z-50">
      <div className="bg-background rounded-lg shadow-lg w-full max-w-md mx-4">
        <div className="flex items-center justify-between p-4 border-b border-border">
          <h2 className="text-lg font-semibold">{t("settings.gitConnections.dialog.title")}</h2>
          <button onClick={onClose} className="text-muted-foreground hover:text-foreground">
            <svg className="w-5 h-5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
              <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M6 18L18 6M6 6l12 12" />
            </svg>
          </button>
        </div>

        <div className="p-4">
          {error && (
            <div className="mb-4 p-3 bg-destructive/10 text-destructive text-sm rounded-lg">
              {error}
            </div>
          )}

          {step === "provider" && (
            <div className="space-y-3">
              <p className="text-sm text-muted-foreground mb-4">
                {t("settings.gitConnections.dialog.selectProvider")}
              </p>
              {providers.map((provider) => (
                <button
                  key={provider.type}
                  onClick={() => selectProvider(provider.type)}
                  className="w-full flex items-center gap-4 p-4 border border-border rounded-lg hover:bg-muted/50 transition-colors"
                >
                  <ProviderIcon provider={provider.type} />
                  <div className="text-left">
                    <div className="font-medium">{provider.name}</div>
                    <div className="text-sm text-muted-foreground">
                      {t(provider.descriptionKey)}
                    </div>
                  </div>
                </button>
              ))}
            </div>
          )}

          {step === "credentials" && (
            <div className="space-y-4">
              <button
                onClick={() => setStep("provider")}
                className="flex items-center gap-1 text-sm text-muted-foreground hover:text-foreground"
              >
                <svg className="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                  <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M15 19l-7-7 7-7" />
                </svg>
                {t("common.back")}
              </button>

              <div>
                <label className="block text-sm font-medium mb-2">
                  {t("settings.gitConnections.dialog.connectionName")}
                </label>
                <Input
                  value={providerName}
                  onChange={(e) => setProviderName(e.target.value)}
                  placeholder={t("settings.gitConnections.dialog.connectionNamePlaceholder")}
                />
              </div>

              <div>
                <label className="block text-sm font-medium mb-2">
                  {t("settings.gitConnections.dialog.baseUrl")}
                </label>
                <Input
                  value={baseURL}
                  onChange={(e) => setBaseURL(e.target.value)}
                  placeholder="https://gitlab.company.com"
                />
                <p className="text-xs text-muted-foreground mt-1">
                  {t("settings.gitConnections.dialog.baseUrlHint")}
                </p>
              </div>

              <div>
                <label className="block text-sm font-medium mb-2">
                  {t("settings.gitConnections.dialog.authType")}
                </label>
                <div className="flex gap-4">
                  <label className="flex items-center gap-2">
                    <input
                      type="radio"
                      checked={authType === "pat"}
                      onChange={() => setAuthType("pat")}
                      className="w-4 h-4"
                    />
                    <span className="text-sm">Personal Access Token</span>
                  </label>
                  <label className="flex items-center gap-2">
                    <input
                      type="radio"
                      checked={authType === "ssh"}
                      onChange={() => setAuthType("ssh")}
                      className="w-4 h-4"
                    />
                    <span className="text-sm">SSH Key</span>
                  </label>
                </div>
              </div>

              {authType === "pat" && (
                <div>
                  <label className="block text-sm font-medium mb-2">
                    Personal Access Token
                  </label>
                  <Input
                    type="password"
                    value={accessToken}
                    onChange={(e) => setAccessToken(e.target.value)}
                    placeholder="ghp_xxxx or glpat-xxxx"
                  />
                  <p className="text-xs text-muted-foreground mt-1">
                    {t("settings.gitConnections.dialog.patHint")}
                  </p>
                </div>
              )}

              {authType === "ssh" && (
                <div>
                  <label className="block text-sm font-medium mb-2">
                    {t("settings.gitConnections.dialog.sshPrivateKey")}
                  </label>
                  <textarea
                    value={sshPrivateKey}
                    onChange={(e) => setSSHPrivateKey(e.target.value)}
                    placeholder="-----BEGIN OPENSSH PRIVATE KEY-----"
                    className="flex min-h-[120px] w-full rounded-md border border-input bg-transparent px-3 py-2 text-sm placeholder:text-muted-foreground focus-visible:outline-none focus-visible:ring-1 focus-visible:ring-ring"
                  />
                </div>
              )}
            </div>
          )}
        </div>

        {step === "credentials" && (
          <div className="flex justify-end gap-3 p-4 border-t border-border">
            <Button variant="outline" onClick={onClose}>
              {t("common.cancel")}
            </Button>
            <Button onClick={handleSubmit} loading={saving}>
              {t("settings.gitConnections.dialog.connect")}
            </Button>
          </div>
        )}
      </div>
    </div>
  );
}

// Repository Browser Dialog
function RepositoryBrowserDialog({
  connection,
  onClose,
}: {
  connection: GitConnectionData;
  onClose: () => void;
}) {
  const t = useTranslations();
  const [repositories, setRepositories] = useState<RemoteRepositoryData[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [search, setSearch] = useState("");
  const [page, setPage] = useState(1);

  const loadRepositories = useCallback(async () => {
    try {
      setLoading(true);
      setError(null);
      const response = await gitConnectionApi.listRepositories(connection.id, {
        page,
        perPage: 20,
        search: search || undefined,
      });
      setRepositories(response.repositories || []);
    } catch (err) {
      console.error("Failed to load repositories:", err);
      setError(t("settings.gitConnections.repoBrowser.failedToLoad"));
    } finally {
      setLoading(false);
    }
  }, [connection.id, page, search, t]);

  useEffect(() => {
    loadRepositories();
  }, [loadRepositories]);

  const handleSearch = (e: React.FormEvent) => {
    e.preventDefault();
    setPage(1);
    loadRepositories();
  };

  return (
    <div className="fixed inset-0 bg-black/50 flex items-center justify-center z-50">
      <div className="bg-background rounded-lg shadow-lg w-full max-w-2xl mx-4 max-h-[80vh] flex flex-col">
        <div className="flex items-center justify-between p-4 border-b border-border">
          <div>
            <h2 className="text-lg font-semibold">{t("settings.gitConnections.repoBrowser.title")}</h2>
            <p className="text-sm text-muted-foreground">
              {connection.provider_name} - {connection.username || connection.base_url}
            </p>
          </div>
          <button onClick={onClose} className="text-muted-foreground hover:text-foreground">
            <svg className="w-5 h-5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
              <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M6 18L18 6M6 6l12 12" />
            </svg>
          </button>
        </div>

        <div className="p-4 border-b border-border">
          <form onSubmit={handleSearch} className="flex gap-2">
            <Input
              value={search}
              onChange={(e) => setSearch(e.target.value)}
              placeholder={t("settings.gitConnections.repoBrowser.searchPlaceholder")}
              className="flex-1"
            />
            <Button type="submit">{t("common.search")}</Button>
          </form>
        </div>

        <div className="flex-1 overflow-auto p-4">
          {error && (
            <div className="mb-4 p-3 bg-destructive/10 text-destructive text-sm rounded-lg">
              {error}
            </div>
          )}

          {loading ? (
            <div className="flex items-center justify-center py-8">
              <div className="animate-spin rounded-full h-8 w-8 border-b-2 border-primary"></div>
            </div>
          ) : repositories.length === 0 ? (
            <p className="text-center text-muted-foreground py-8">
              {t("settings.gitConnections.repoBrowser.noRepos")}
            </p>
          ) : (
            <div className="space-y-2">
              {repositories.map((repo) => (
                <div
                  key={repo.id}
                  className="flex items-center justify-between p-3 border border-border rounded-lg hover:bg-muted/50"
                >
                  <div>
                    <div className="font-medium">{repo.full_path}</div>
                    <div className="text-sm text-muted-foreground line-clamp-1">
                      {repo.description || t("settings.gitConnections.repoBrowser.noDescription")}
                    </div>
                    <div className="flex items-center gap-3 mt-1 text-xs text-muted-foreground">
                      <span className="px-2 py-0.5 bg-muted rounded">
                        {repo.visibility}
                      </span>
                      <span>{t("settings.gitConnections.repoBrowser.branch")}: {repo.default_branch}</span>
                    </div>
                  </div>
                  <a
                    href={repo.web_url}
                    target="_blank"
                    rel="noopener noreferrer"
                    className="text-muted-foreground hover:text-foreground"
                  >
                    <svg className="w-5 h-5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                      <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M10 6H6a2 2 0 00-2 2v10a2 2 0 002 2h10a2 2 0 002-2v-4M14 4h6m0 0v6m0-6L10 14" />
                    </svg>
                  </a>
                </div>
              ))}
            </div>
          )}
        </div>

        <div className="flex items-center justify-between p-4 border-t border-border">
          <Button
            variant="outline"
            size="sm"
            disabled={page <= 1}
            onClick={() => setPage((p) => p - 1)}
          >
            {t("settings.gitConnections.repoBrowser.previous")}
          </Button>
          <span className="text-sm text-muted-foreground">{t("settings.gitConnections.repoBrowser.page", { page })}</span>
          <Button
            variant="outline"
            size="sm"
            disabled={repositories.length < 20}
            onClick={() => setPage((p) => p + 1)}
          >
            {t("settings.gitConnections.repoBrowser.next")}
          </Button>
        </div>
      </div>
    </div>
  );
}
