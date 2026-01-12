"use client";

import { useState, useEffect, useCallback } from "react";
import { useSearchParams } from "next/navigation";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { useAuthStore } from "@/stores/auth";
import { organizationApi, agentApi, billingApi, BillingOverview, SubscriptionPlan, gitProviderApi, sshKeyApi, SSHKeyData, RedeemPromoCodeResponse } from "@/lib/api/client";
import { PromoCodeInput } from "@/components/promo-code/PromoCodeInput";
import { useRunnerStore, Runner, RegistrationToken, getRunnerStatusInfo } from "@/stores/runner";
import { NotificationSettings, LanguageSettings } from "@/components/settings";
import { useTranslations } from "@/lib/i18n/client";

export default function SettingsPage() {
  const searchParams = useSearchParams();
  const activeTab = searchParams.get("tab") || "general";
  const { currentOrg } = useAuthStore();
  const t = useTranslations();

  // Tab content mapping
  const renderContent = () => {
    switch (activeTab) {
      case "general":
        return <GeneralSettings org={currentOrg} t={t} />;
      case "members":
        return <MembersSettings t={t} />;
      case "agents":
        return <AgentsSettings t={t} />;
      case "runners":
        return <RunnersSettings t={t} />;
      case "git-providers":
        return <GitProvidersSettings t={t} />;
      case "notifications":
        return <NotificationsSettings t={t} />;
      case "billing":
        return <BillingSettings t={t} />;
      default:
        return <GeneralSettings org={currentOrg} t={t} />;
    }
  };

  return (
    <div className="h-full overflow-auto p-6">
      {/* Content - navigation controlled by IDE Sidebar */}
      <div className="max-w-4xl">
        {renderContent()}
      </div>
    </div>
  );
}

type TranslationFn = (key: string, params?: Record<string, string | number>) => string;

function GeneralSettings({ org, t }: { org: { name: string; slug: string } | null; t: TranslationFn }) {
  const [name, setName] = useState(org?.name || "");
  const [saving, setSaving] = useState(false);

  const handleSave = async () => {
    setSaving(true);
    try {
      await organizationApi.update(org!.slug, { name });
    } catch (error) {
      console.error("Failed to save:", error);
    } finally {
      setSaving(false);
    }
  };

  return (
    <div className="space-y-6">
      {/* Language Settings */}
      <LanguageSettings />

      <div className="border border-border rounded-lg p-6">
        <h2 className="text-lg font-semibold mb-4">{t("settings.organizationDetails.title")}</h2>
        <div className="space-y-4">
          <div>
            <label className="block text-sm font-medium mb-2">
              {t("settings.organizationDetails.nameLabel")}
            </label>
            <Input
              value={name}
              onChange={(e) => setName(e.target.value)}
              placeholder={t("settings.organizationDetails.namePlaceholder")}
            />
          </div>
          <div>
            <label className="block text-sm font-medium mb-2">
              {t("settings.organizationDetails.slugLabel")}
            </label>
            <Input value={org?.slug || ""} disabled />
            <p className="text-xs text-muted-foreground mt-1">
              {t("settings.organizationDetails.slugHint")}
            </p>
          </div>
        </div>
        <div className="mt-6">
          <Button onClick={handleSave} disabled={saving}>
            {saving ? t("settings.organizationDetails.saving") : t("settings.organizationDetails.saveChanges")}
          </Button>
        </div>
      </div>

      <div className="border border-destructive rounded-lg p-6">
        <h2 className="text-lg font-semibold text-destructive mb-4">
          {t("settings.dangerZone.title")}
        </h2>
        <p className="text-sm text-muted-foreground mb-4">
          {t("settings.dangerZone.description")}
        </p>
        <Button variant="destructive">{t("settings.dangerZone.deleteOrg")}</Button>
      </div>
    </div>
  );
}

function MembersSettings({ t }: { t: TranslationFn }) {
  const { currentOrg, user } = useAuthStore();
  const [members, setMembers] = useState<Array<{
    id: number;
    user_id: number;
    role: string;
    joined_at: string;
    user?: { id: number; email: string; username: string; name?: string };
  }>>([]);
  const [loading, setLoading] = useState(true);
  const [showInviteDialog, setShowInviteDialog] = useState(false);
  const [inviteEmail, setInviteEmail] = useState("");
  const [inviteRole, setInviteRole] = useState("member");
  const [inviting, setInviting] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const loadMembers = useCallback(async () => {
    if (!currentOrg) return;
    try {
      setLoading(true);
      const response = await organizationApi.listMembers(currentOrg.slug);
      setMembers(response.members || []);
    } catch (err) {
      console.error("Failed to load members:", err);
      setError("Failed to load members");
    } finally {
      setLoading(false);
    }
  }, [currentOrg]);

  useEffect(() => {
    loadMembers();
  }, [loadMembers]);

  const handleInvite = async () => {
    if (!currentOrg || !inviteEmail) return;
    setInviting(true);
    setError(null);
    try {
      await organizationApi.inviteMember(currentOrg.slug, inviteEmail, inviteRole);
      setShowInviteDialog(false);
      setInviteEmail("");
      setInviteRole("member");
      await loadMembers();
    } catch (err) {
      console.error("Failed to invite member:", err);
      setError("Failed to invite member. Please check the email and try again.");
    } finally {
      setInviting(false);
    }
  };

  const handleRemove = async (userId: number) => {
    if (!currentOrg) return;
    if (!confirm("Are you sure you want to remove this member?")) return;
    try {
      await organizationApi.removeMember(currentOrg.slug, userId);
      await loadMembers();
    } catch (err) {
      console.error("Failed to remove member:", err);
      setError("Failed to remove member");
    }
  };

  const handleRoleChange = async (userId: number, newRole: string) => {
    if (!currentOrg) return;
    try {
      await organizationApi.updateMemberRole(currentOrg.slug, userId, newRole);
      await loadMembers();
    } catch (err) {
      console.error("Failed to update role:", err);
      setError("Failed to update member role");
    }
  };

  const getRoleBadgeColor = (role: string) => {
    switch (role) {
      case "owner": return "bg-purple-100 text-purple-800";
      case "admin": return "bg-blue-100 text-blue-800";
      default: return "bg-gray-100 text-gray-800";
    }
  };

  return (
    <div className="border border-border rounded-lg p-6">
      <div className="flex items-center justify-between mb-4">
        <div>
          <h2 className="text-lg font-semibold">{t("settings.members.title")}</h2>
          <p className="text-sm text-muted-foreground">
            {t("settings.members.description")}
          </p>
        </div>
        <Button onClick={() => setShowInviteDialog(true)}>{t("settings.members.inviteMember")}</Button>
      </div>

      {error && (
        <div className="bg-destructive/10 border border-destructive text-destructive px-4 py-3 rounded-lg mb-4">
          {error}
          <button onClick={() => setError(null)} className="ml-4 underline text-sm">
            {t("settings.members.dismiss")}
          </button>
        </div>
      )}

      {loading ? (
        <div className="text-center py-8 text-muted-foreground">{t("settings.members.loading")}</div>
      ) : members.length === 0 ? (
        <div className="text-center py-8 text-muted-foreground">
          {t("settings.members.noMembers")}
        </div>
      ) : (
        <div className="space-y-3">
          {members.map((member) => (
            <div
              key={member.id}
              className="flex items-center justify-between p-4 border border-border rounded-lg"
            >
              <div className="flex items-center gap-3">
                <div className="w-10 h-10 rounded-full bg-muted flex items-center justify-center text-sm font-medium">
                  {member.user?.name?.[0] || member.user?.username?.[0] || "?"}
                </div>
                <div>
                  <div className="flex items-center gap-2">
                    <span className="font-medium">
                      {member.user?.name || member.user?.username || "Unknown"}
                    </span>
                    <span className={`text-xs px-2 py-0.5 rounded-full ${getRoleBadgeColor(member.role)}`}>
                      {member.role}
                    </span>
                    {member.user_id === user?.id && (
                      <span className="text-xs text-muted-foreground">{t("settings.members.you")}</span>
                    )}
                  </div>
                  <p className="text-sm text-muted-foreground">{member.user?.email}</p>
                </div>
              </div>
              <div className="flex items-center gap-2">
                {member.role !== "owner" && member.user_id !== user?.id && (
                  <>
                    <select
                      value={member.role}
                      onChange={(e) => handleRoleChange(member.user_id, e.target.value)}
                      className="text-sm border border-border rounded px-2 py-1 bg-background"
                    >
                      <option value="member">{t("settings.members.roleMember")}</option>
                      <option value="admin">{t("settings.members.roleAdmin")}</option>
                    </select>
                    <Button
                      variant="ghost"
                      size="sm"
                      className="text-destructive hover:text-destructive"
                      onClick={() => handleRemove(member.user_id)}
                    >
                      {t("settings.members.remove")}
                    </Button>
                  </>
                )}
              </div>
            </div>
          ))}
        </div>
      )}

      {/* Invite Dialog */}
      {showInviteDialog && (
        <div className="fixed inset-0 bg-black/50 flex items-center justify-center z-50">
          <div className="bg-background border border-border rounded-lg p-6 w-full max-w-md">
            <h3 className="text-lg font-semibold mb-4">{t("settings.members.inviteDialog.title")}</h3>
            <div className="space-y-4">
              <div>
                <label className="block text-sm font-medium mb-2">{t("settings.members.inviteDialog.emailLabel")}</label>
                <Input
                  type="email"
                  value={inviteEmail}
                  onChange={(e) => setInviteEmail(e.target.value)}
                  placeholder={t("settings.members.inviteDialog.emailPlaceholder")}
                />
              </div>
              <div>
                <label className="block text-sm font-medium mb-2">{t("settings.members.inviteDialog.roleLabel")}</label>
                <select
                  value={inviteRole}
                  onChange={(e) => setInviteRole(e.target.value)}
                  className="w-full border border-border rounded px-3 py-2 bg-background"
                >
                  <option value="member">{t("settings.members.roleMember")}</option>
                  <option value="admin">{t("settings.members.roleAdmin")}</option>
                </select>
              </div>
            </div>
            <div className="flex gap-3 mt-6">
              <Button
                variant="outline"
                className="flex-1"
                onClick={() => {
                  setShowInviteDialog(false);
                  setInviteEmail("");
                  setInviteRole("member");
                }}
              >
                {t("settings.members.inviteDialog.cancel")}
              </Button>
              <Button
                className="flex-1"
                onClick={handleInvite}
                disabled={inviting || !inviteEmail}
              >
                {inviting ? t("settings.members.inviteDialog.inviting") : t("settings.members.inviteDialog.sendInvite")}
              </Button>
            </div>
          </div>
        </div>
      )}
    </div>
  );
}

function AgentsSettings({ t }: { t: TranslationFn }) {
  const [agentTypes, setAgentTypes] = useState<
    Array<{ id: number; slug: string; name: string; description?: string }>
  >([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [success, setSuccess] = useState<string | null>(null);

  // Credentials state
  const [anthropicKey, setAnthropicKey] = useState("");
  const [openaiKey, setOpenaiKey] = useState("");
  const [googleKey, setGoogleKey] = useState("");
  const [savingCredentials, setSavingCredentials] = useState(false);

  useEffect(() => {
    loadAgentTypes();
  }, []);

  const loadAgentTypes = async () => {
    try {
      const response = await agentApi.listTypes();
      setAgentTypes(response.agent_types || []);
    } catch (error) {
      console.error("Failed to load agent types:", error);
    } finally {
      setLoading(false);
    }
  };

  const handleSaveCredentials = async () => {
    setSavingCredentials(true);
    setError(null);
    setSuccess(null);
    try {
      // Save credentials for each provider that has a value
      const promises = [];
      if (anthropicKey) {
        promises.push(
          agentApi.updateCredentials("claude", { api_key: anthropicKey })
        );
      }
      if (openaiKey) {
        promises.push(
          agentApi.updateCredentials("openai", { api_key: openaiKey })
        );
      }
      if (googleKey) {
        promises.push(
          agentApi.updateCredentials("gemini", { api_key: googleKey })
        );
      }

      if (promises.length === 0) {
        setError("Please enter at least one API key to save");
        return;
      }

      await Promise.all(promises);
      setSuccess("Credentials saved successfully");
      // Clear the inputs after saving
      setAnthropicKey("");
      setOpenaiKey("");
      setGoogleKey("");
    } catch (err) {
      console.error("Failed to save credentials:", err);
      setError("Failed to save credentials. Please try again.");
    } finally {
      setSavingCredentials(false);
    }
  };

  return (
    <div className="space-y-6">
      <div className="border border-border rounded-lg p-6">
        <h2 className="text-lg font-semibold mb-4">{t("settings.agentConfig.title")}</h2>
        <p className="text-sm text-muted-foreground mb-4">
          {t("settings.agentConfig.description")}
        </p>

        {loading ? (
          <div className="text-center py-4">{t("settings.agentConfig.loading")}</div>
        ) : (
          <div className="space-y-4">
            {agentTypes.map((agent) => (
              <div
                key={agent.id}
                className="flex items-center justify-between p-4 border border-border rounded-lg"
              >
                <div>
                  <h3 className="font-medium">{agent.name}</h3>
                  <p className="text-sm text-muted-foreground">
                    {agent.description || t("settings.agentConfig.configureDefault", { name: agent.name })}
                  </p>
                </div>
                <div className="flex items-center gap-2">
                  <Button variant="outline" size="sm">
                    {t("settings.agentConfig.configure")}
                  </Button>
                  <label className="relative inline-flex items-center cursor-pointer">
                    <input type="checkbox" className="sr-only peer" />
                    <div className="w-11 h-6 bg-gray-200 peer-focus:outline-none rounded-full peer peer-checked:after:translate-x-full peer-checked:after:border-white after:content-[''] after:absolute after:top-[2px] after:left-[2px] after:bg-white after:border-gray-300 after:border after:rounded-full after:h-5 after:w-5 after:transition-all peer-checked:bg-primary"></div>
                  </label>
                </div>
              </div>
            ))}
          </div>
        )}
      </div>

      <div className="border border-border rounded-lg p-6">
        <h2 className="text-lg font-semibold mb-4">{t("settings.credentials.title")}</h2>
        <p className="text-sm text-muted-foreground mb-4">
          {t("settings.credentials.description")}
        </p>

        {error && (
          <div className="bg-destructive/10 border border-destructive text-destructive px-4 py-3 rounded-lg mb-4">
            {error}
            <button onClick={() => setError(null)} className="ml-4 underline text-sm">
              {t("settings.credentials.dismiss")}
            </button>
          </div>
        )}

        {success && (
          <div className="bg-green-50 border border-green-500 text-green-700 px-4 py-3 rounded-lg mb-4">
            {success}
            <button onClick={() => setSuccess(null)} className="ml-4 underline text-sm">
              {t("settings.credentials.dismiss")}
            </button>
          </div>
        )}

        <div className="space-y-4">
          <div>
            <label className="block text-sm font-medium mb-2">
              {t("settings.credentials.anthropicLabel")}
            </label>
            <Input
              type="password"
              placeholder={t("settings.credentials.anthropicPlaceholder")}
              value={anthropicKey}
              onChange={(e) => setAnthropicKey(e.target.value)}
            />
            <p className="text-xs text-muted-foreground mt-1">
              {t("settings.credentials.anthropicHint")}{" "}
              <a
                href="https://console.anthropic.com/settings/keys"
                target="_blank"
                rel="noopener noreferrer"
                className="text-primary hover:underline"
              >
                console.anthropic.com
              </a>
            </p>
          </div>
          <div>
            <label className="block text-sm font-medium mb-2">
              {t("settings.credentials.openaiLabel")}
            </label>
            <Input
              type="password"
              placeholder={t("settings.credentials.openaiPlaceholder")}
              value={openaiKey}
              onChange={(e) => setOpenaiKey(e.target.value)}
            />
            <p className="text-xs text-muted-foreground mt-1">
              {t("settings.credentials.openaiHint")}{" "}
              <a
                href="https://platform.openai.com/api-keys"
                target="_blank"
                rel="noopener noreferrer"
                className="text-primary hover:underline"
              >
                platform.openai.com
              </a>
            </p>
          </div>
          <div>
            <label className="block text-sm font-medium mb-2">
              {t("settings.credentials.googleLabel")}
            </label>
            <Input
              type="password"
              placeholder={t("settings.credentials.googlePlaceholder")}
              value={googleKey}
              onChange={(e) => setGoogleKey(e.target.value)}
            />
            <p className="text-xs text-muted-foreground mt-1">
              {t("settings.credentials.googleHint")}{" "}
              <a
                href="https://aistudio.google.com/app/apikey"
                target="_blank"
                rel="noopener noreferrer"
                className="text-primary hover:underline"
              >
                aistudio.google.com
              </a>
            </p>
          </div>
        </div>
        <div className="mt-4">
          <Button
            onClick={handleSaveCredentials}
            disabled={savingCredentials || (!anthropicKey && !openaiKey && !googleKey)}
          >
            {savingCredentials ? t("settings.credentials.saving") : t("settings.credentials.saveCredentials")}
          </Button>
        </div>
      </div>
    </div>
  );
}

function GitProvidersSettings({ t }: { t: TranslationFn }) {
  const [providers, setProviders] = useState<Array<{
    id: number;
    provider_type: string;
    name: string;
    base_url: string;
    ssh_key_id?: number;
    is_default: boolean;
    is_active: boolean;
  }>>([]);
  const [sshKeys, setSSHKeys] = useState<SSHKeyData[]>([]);
  const [loading, setLoading] = useState(true);
  const [showAddDialog, setShowAddDialog] = useState(false);
  const [editingProvider, setEditingProvider] = useState<number | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [success, setSuccess] = useState<string | null>(null);

  // SSH Key management
  const [showSSHKeyDialog, setShowSSHKeyDialog] = useState(false);
  const [newSSHKeyName, setNewSSHKeyName] = useState("");
  const [newSSHKeyPrivate, setNewSSHKeyPrivate] = useState("");
  const [createdSSHKey, setCreatedSSHKey] = useState<SSHKeyData | null>(null);
  const [savingSSHKey, setSavingSSHKey] = useState(false);

  // Form states
  const [formType, setFormType] = useState("github");
  const [formName, setFormName] = useState("");
  const [formBaseUrl, setFormBaseUrl] = useState("");
  const [formClientId, setFormClientId] = useState("");
  const [formClientSecret, setFormClientSecret] = useState("");
  const [formBotToken, setFormBotToken] = useState("");
  const [formSSHKeyId, setFormSSHKeyId] = useState<number | null>(null);
  const [formIsDefault, setFormIsDefault] = useState(false);
  const [saving, setSaving] = useState(false);

  const loadData = useCallback(async () => {
    try {
      setLoading(true);
      const [providersRes, sshKeysRes] = await Promise.all([
        gitProviderApi.list(),
        sshKeyApi.list(),
      ]);
      setProviders(providersRes.git_providers || []);
      setSSHKeys(sshKeysRes.ssh_keys || []);
    } catch (err) {
      console.error("Failed to load data:", err);
      setError("Failed to load git providers");
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    loadData();
  }, [loadData]);

  const getDefaultBaseUrl = (type: string) => {
    switch (type) {
      case "github": return "https://github.com";
      case "gitlab": return "https://gitlab.com";
      case "gitee": return "https://gitee.com";
      case "ssh": return "";
      default: return "";
    }
  };

  const resetForm = () => {
    setFormType("github");
    setFormName("");
    setFormBaseUrl("");
    setFormClientId("");
    setFormClientSecret("");
    setFormBotToken("");
    setFormSSHKeyId(null);
    setFormIsDefault(false);
    setEditingProvider(null);
  };

  const handleAddProvider = async () => {
    setSaving(true);
    setError(null);
    try {
      await gitProviderApi.create({
        provider_type: formType,
        name: formName || `${formType.charAt(0).toUpperCase() + formType.slice(1)}`,
        base_url: formBaseUrl || getDefaultBaseUrl(formType),
        client_id: formClientId || undefined,
        client_secret: formClientSecret || undefined,
        bot_token: formBotToken || undefined,
        ssh_key_id: formType === "ssh" && formSSHKeyId ? formSSHKeyId : undefined,
        is_default: formIsDefault,
      });
      setShowAddDialog(false);
      resetForm();
      await loadData();
    } catch (err) {
      console.error("Failed to add provider:", err);
      setError("Failed to add provider");
    } finally {
      setSaving(false);
    }
  };

  const handleUpdateProvider = async (id: number) => {
    setSaving(true);
    setError(null);
    try {
      await gitProviderApi.update(id, {
        name: formName || undefined,
        base_url: formBaseUrl || undefined,
        client_id: formClientId || undefined,
        client_secret: formClientSecret || undefined,
        bot_token: formBotToken || undefined,
        ssh_key_id: formType === "ssh" && formSSHKeyId ? formSSHKeyId : undefined,
        is_default: formIsDefault,
      });
      setEditingProvider(null);
      resetForm();
      await loadData();
    } catch (err) {
      console.error("Failed to update provider:", err);
      setError("Failed to update provider");
    } finally {
      setSaving(false);
    }
  };

  const handleDeleteProvider = async (id: number) => {
    if (!confirm("Are you sure you want to delete this provider?")) return;
    try {
      await gitProviderApi.delete(id);
      await loadData();
    } catch (err) {
      console.error("Failed to delete provider:", err);
      setError("Failed to delete provider");
    }
  };

  const handleToggleActive = async (id: number, isActive: boolean) => {
    try {
      await gitProviderApi.update(id, { is_active: !isActive });
      await loadData();
    } catch (err) {
      console.error("Failed to toggle provider:", err);
      setError("Failed to toggle provider status");
    }
  };

  const openEditDialog = (provider: typeof providers[0]) => {
    setFormType(provider.provider_type);
    setFormName(provider.name);
    setFormBaseUrl(provider.base_url);
    setFormSSHKeyId(provider.ssh_key_id || null);
    setFormIsDefault(provider.is_default);
    setEditingProvider(provider.id);
  };

  const handleCreateSSHKey = async () => {
    if (!newSSHKeyName) {
      setError("SSH key name is required");
      return;
    }
    setSavingSSHKey(true);
    setError(null);
    try {
      const res = await sshKeyApi.create({
        name: newSSHKeyName,
        private_key: newSSHKeyPrivate || undefined, // If empty, generate new key pair
      });
      setCreatedSSHKey(res.ssh_key);
      setSuccess("SSH key created successfully");
      await loadData();
    } catch (err) {
      console.error("Failed to create SSH key:", err);
      setError("Failed to create SSH key");
    } finally {
      setSavingSSHKey(false);
    }
  };

  const handleDeleteSSHKey = async (id: number) => {
    if (!confirm("Are you sure you want to delete this SSH key?")) return;
    try {
      await sshKeyApi.delete(id);
      await loadData();
      setSuccess("SSH key deleted");
    } catch (err) {
      console.error("Failed to delete SSH key:", err);
      setError("Failed to delete SSH key");
    }
  };

  const resetSSHKeyDialog = () => {
    setShowSSHKeyDialog(false);
    setNewSSHKeyName("");
    setNewSSHKeyPrivate("");
    setCreatedSSHKey(null);
  };

  const copyToClipboard = (text: string) => {
    navigator.clipboard.writeText(text);
  };

  const getProviderIcon = (type: string) => {
    switch (type) {
      case "github": return "GH";
      case "gitlab": return "GL";
      case "gitee": return "GE";
      case "ssh": return "🔑";
      default: return "?";
    }
  };

  const isSSHProvider = formType === "ssh";

  return (
    <div className="space-y-6">
      {/* Git Providers */}
      <div className="border border-border rounded-lg p-6">
        <div className="flex items-center justify-between mb-4">
          <div>
            <h2 className="text-lg font-semibold">{t("settings.gitProviders.title")}</h2>
            <p className="text-sm text-muted-foreground">
              {t("settings.gitProviders.description")}
            </p>
          </div>
          <Button onClick={() => setShowAddDialog(true)}>{t("settings.gitProviders.addProvider")}</Button>
        </div>

        {error && (
          <div className="bg-destructive/10 border border-destructive text-destructive px-4 py-3 rounded-lg mb-4">
            {error}
            <button onClick={() => setError(null)} className="ml-4 underline text-sm">
              {t("settings.members.dismiss")}
            </button>
          </div>
        )}

        {success && (
          <div className="bg-green-50 border border-green-500 text-green-700 px-4 py-3 rounded-lg mb-4">
            {success}
            <button onClick={() => setSuccess(null)} className="ml-4 underline text-sm">
              {t("settings.members.dismiss")}
            </button>
          </div>
        )}

        {loading ? (
          <div className="text-center py-8 text-muted-foreground">{t("settings.gitProviders.loading")}</div>
        ) : providers.length === 0 ? (
          <div className="text-center py-8 text-muted-foreground">
            {t("settings.gitProviders.noProviders")}
          </div>
        ) : (
          <div className="space-y-4">
            {providers.map((provider) => (
              <div
                key={provider.id}
                className={`flex items-center justify-between p-4 border border-border rounded-lg ${
                  !provider.is_active ? "opacity-60" : ""
                }`}
              >
                <div className="flex items-center gap-3">
                  <div className="w-10 h-10 rounded-lg bg-muted flex items-center justify-center font-medium">
                    {getProviderIcon(provider.provider_type)}
                  </div>
                  <div>
                    <div className="flex items-center gap-2">
                      <h3 className="font-medium">{provider.name}</h3>
                      <span className="text-xs bg-muted px-2 py-0.5 rounded">
                        {provider.provider_type.toUpperCase()}
                      </span>
                      {provider.is_default && (
                        <span className="text-xs bg-primary/10 text-primary px-2 py-0.5 rounded">
                          {t("settings.gitProviders.default")}
                        </span>
                      )}
                      {!provider.is_active && (
                        <span className="text-xs bg-yellow-100 text-yellow-800 px-2 py-0.5 rounded">
                          {t("settings.gitProviders.disabled")}
                        </span>
                      )}
                    </div>
                    <p className="text-sm text-muted-foreground">
                      {provider.base_url || (provider.provider_type === "ssh" ? t("settings.gitProviders.sshAuth") : "")}
                    </p>
                  </div>
                </div>
                <div className="flex items-center gap-2">
                  <Button variant="outline" size="sm" onClick={() => openEditDialog(provider)}>
                    {t("settings.gitProviders.configure")}
                  </Button>
                  <Button
                    variant="ghost"
                    size="sm"
                    onClick={() => handleToggleActive(provider.id, provider.is_active)}
                  >
                    {provider.is_active ? t("settings.gitProviders.disable") : t("settings.gitProviders.enable")}
                  </Button>
                  <Button
                    variant="ghost"
                    size="sm"
                    className="text-destructive hover:text-destructive"
                    onClick={() => handleDeleteProvider(provider.id)}
                  >
                    {t("settings.gitProviders.delete")}
                  </Button>
                </div>
              </div>
            ))}
          </div>
        )}

        {/* Add/Edit Dialog */}
        {(showAddDialog || editingProvider !== null) && (
          <div className="fixed inset-0 bg-black/50 flex items-center justify-center z-50">
            <div className="bg-background border border-border rounded-lg p-6 w-full max-w-md max-h-[90vh] overflow-y-auto">
              <h3 className="text-lg font-semibold mb-4">
                {editingProvider ? t("settings.gitProviders.dialog.editTitle") : t("settings.gitProviders.dialog.addTitle")}
              </h3>
              <div className="space-y-4">
                {!editingProvider && (
                  <div>
                    <label className="block text-sm font-medium mb-2">{t("settings.gitProviders.dialog.providerType")}</label>
                    <select
                      value={formType}
                      onChange={(e) => {
                        setFormType(e.target.value);
                        setFormBaseUrl(getDefaultBaseUrl(e.target.value));
                      }}
                      className="w-full border border-border rounded px-3 py-2 bg-background"
                    >
                      <option value="github">GitHub</option>
                      <option value="gitlab">GitLab</option>
                      <option value="gitee">Gitee</option>
                      <option value="ssh">SSH (Generic)</option>
                    </select>
                    {isSSHProvider && (
                      <p className="text-xs text-muted-foreground mt-1">
                        {t("settings.gitProviders.dialog.sshHint")}
                      </p>
                    )}
                  </div>
                )}
                <div>
                  <label className="block text-sm font-medium mb-2">{t("settings.gitProviders.dialog.nameLabel")}</label>
                  <Input
                    value={formName}
                    onChange={(e) => setFormName(e.target.value)}
                    placeholder={isSSHProvider ? t("settings.gitProviders.dialog.namePlaceholderSSH") : t("settings.gitProviders.dialog.namePlaceholder")}
                  />
                </div>
                {!isSSHProvider && (
                  <>
                    <div>
                      <label className="block text-sm font-medium mb-2">{t("settings.gitProviders.dialog.baseUrlLabel")}</label>
                      <Input
                        value={formBaseUrl}
                        onChange={(e) => setFormBaseUrl(e.target.value)}
                        placeholder={getDefaultBaseUrl(formType)}
                      />
                      <p className="text-xs text-muted-foreground mt-1">
                        {t("settings.gitProviders.dialog.baseUrlHint")}
                      </p>
                    </div>
                    <div>
                      <label className="block text-sm font-medium mb-2">{t("settings.gitProviders.dialog.clientIdLabel")}</label>
                      <Input
                        value={formClientId}
                        onChange={(e) => setFormClientId(e.target.value)}
                        placeholder={t("settings.gitProviders.dialog.clientIdPlaceholder")}
                      />
                    </div>
                    <div>
                      <label className="block text-sm font-medium mb-2">{t("settings.gitProviders.dialog.clientSecretLabel")}</label>
                      <Input
                        type="password"
                        value={formClientSecret}
                        onChange={(e) => setFormClientSecret(e.target.value)}
                        placeholder={t("settings.gitProviders.dialog.clientSecretPlaceholder")}
                      />
                    </div>
                    <div>
                      <label className="block text-sm font-medium mb-2">{t("settings.gitProviders.dialog.botTokenLabel")}</label>
                      <Input
                        type="password"
                        value={formBotToken}
                        onChange={(e) => setFormBotToken(e.target.value)}
                        placeholder={t("settings.gitProviders.dialog.botTokenPlaceholder")}
                      />
                    </div>
                  </>
                )}
                {isSSHProvider && (
                  <div>
                    <label className="block text-sm font-medium mb-2">{t("settings.gitProviders.dialog.sshKeyLabel")}</label>
                    <select
                      value={formSSHKeyId || ""}
                      onChange={(e) => setFormSSHKeyId(e.target.value ? Number(e.target.value) : null)}
                      className="w-full border border-border rounded px-3 py-2 bg-background"
                    >
                      <option value="">{t("settings.gitProviders.dialog.sshKeyPlaceholder")}</option>
                      {sshKeys.map((key) => (
                        <option key={key.id} value={key.id}>
                          {key.name} ({key.fingerprint.substring(0, 16)}...)
                        </option>
                      ))}
                    </select>
                    {sshKeys.length === 0 && (
                      <p className="text-xs text-muted-foreground mt-1">
                        {t("settings.gitProviders.dialog.noSSHKeys")}
                      </p>
                    )}
                  </div>
                )}
                <div className="flex items-center gap-2">
                  <input
                    type="checkbox"
                    id="isDefault"
                    checked={formIsDefault}
                    onChange={(e) => setFormIsDefault(e.target.checked)}
                  />
                  <label htmlFor="isDefault" className="text-sm">{t("settings.gitProviders.dialog.setDefault")}</label>
                </div>
              </div>
              <div className="flex gap-3 mt-6">
                <Button
                  variant="outline"
                  className="flex-1"
                  onClick={() => {
                    setShowAddDialog(false);
                    resetForm();
                  }}
                >
                  {t("settings.gitProviders.dialog.cancel")}
                </Button>
                <Button
                  className="flex-1"
                  onClick={() => editingProvider ? handleUpdateProvider(editingProvider) : handleAddProvider()}
                  disabled={saving || (isSSHProvider && !formSSHKeyId)}
                >
                  {saving ? t("settings.gitProviders.dialog.saving") : editingProvider ? t("settings.gitProviders.dialog.saveChanges") : t("settings.gitProviders.dialog.addProvider")}
                </Button>
              </div>
            </div>
          </div>
        )}
      </div>

      {/* SSH Keys Management */}
      <div className="border border-border rounded-lg p-6">
        <div className="flex items-center justify-between mb-4">
          <div>
            <h2 className="text-lg font-semibold">{t("settings.sshKeys.title")}</h2>
            <p className="text-sm text-muted-foreground">
              {t("settings.sshKeys.description")}
            </p>
          </div>
          <Button onClick={() => setShowSSHKeyDialog(true)}>{t("settings.sshKeys.addSSHKey")}</Button>
        </div>

        {sshKeys.length === 0 ? (
          <div className="text-center py-8 text-muted-foreground">
            {t("settings.sshKeys.noKeys")}
          </div>
        ) : (
          <div className="space-y-4">
            {sshKeys.map((key) => (
              <div
                key={key.id}
                className="flex items-center justify-between p-4 border border-border rounded-lg"
              >
                <div className="flex items-center gap-3">
                  <div className="w-10 h-10 rounded-lg bg-muted flex items-center justify-center">
                    🔑
                  </div>
                  <div>
                    <h3 className="font-medium">{key.name}</h3>
                    <p className="text-xs text-muted-foreground font-mono">
                      {key.fingerprint}
                    </p>
                  </div>
                </div>
                <div className="flex items-center gap-2">
                  <Button
                    variant="outline"
                    size="sm"
                    onClick={() => copyToClipboard(key.public_key)}
                  >
                    {t("settings.sshKeys.copyPublicKey")}
                  </Button>
                  <Button
                    variant="ghost"
                    size="sm"
                    className="text-destructive hover:text-destructive"
                    onClick={() => handleDeleteSSHKey(key.id)}
                  >
                    {t("settings.sshKeys.delete")}
                  </Button>
                </div>
              </div>
            ))}
          </div>
        )}

        {/* Add SSH Key Dialog */}
        {showSSHKeyDialog && (
          <div className="fixed inset-0 bg-black/50 flex items-center justify-center z-50">
            <div className="bg-background border border-border rounded-lg p-6 w-full max-w-lg">
              {createdSSHKey ? (
                <>
                  <h3 className="text-lg font-semibold mb-4">{t("settings.sshKeys.dialog.createdTitle")}</h3>
                  <p className="text-sm text-muted-foreground mb-4">
                    {t("settings.sshKeys.dialog.createdHint")}
                  </p>
                  <div className="bg-muted p-3 rounded-lg mb-4">
                    <code className="text-xs break-all">{createdSSHKey.public_key}</code>
                  </div>
                  <div className="flex gap-3">
                    <Button
                      variant="outline"
                      className="flex-1"
                      onClick={() => copyToClipboard(createdSSHKey.public_key)}
                    >
                      {t("settings.sshKeys.copyPublicKey")}
                    </Button>
                    <Button className="flex-1" onClick={resetSSHKeyDialog}>
                      {t("settings.sshKeys.dialog.done")}
                    </Button>
                  </div>
                </>
              ) : (
                <>
                  <h3 className="text-lg font-semibold mb-4">{t("settings.sshKeys.dialog.addTitle")}</h3>
                  <div className="space-y-4">
                    <div>
                      <label className="block text-sm font-medium mb-2">{t("settings.sshKeys.dialog.nameLabel")}</label>
                      <Input
                        value={newSSHKeyName}
                        onChange={(e) => setNewSSHKeyName(e.target.value)}
                        placeholder={t("settings.sshKeys.dialog.namePlaceholder")}
                      />
                    </div>
                    <div>
                      <label className="block text-sm font-medium mb-2">
                        {t("settings.sshKeys.dialog.privateKeyLabel")}
                      </label>
                      <textarea
                        className="w-full px-3 py-2 border border-border rounded-md bg-background font-mono text-xs"
                        rows={6}
                        value={newSSHKeyPrivate}
                        onChange={(e) => setNewSSHKeyPrivate(e.target.value)}
                        placeholder={t("settings.sshKeys.dialog.privateKeyPlaceholder")}
                      />
                      <p className="text-xs text-muted-foreground mt-1">
                        {t("settings.sshKeys.dialog.privateKeyHint")}
                      </p>
                    </div>
                  </div>
                  <div className="flex gap-3 mt-6">
                    <Button variant="outline" className="flex-1" onClick={resetSSHKeyDialog}>
                      {t("settings.sshKeys.dialog.cancel")}
                    </Button>
                    <Button
                      className="flex-1"
                      onClick={handleCreateSSHKey}
                      disabled={savingSSHKey || !newSSHKeyName}
                    >
                      {savingSSHKey ? t("settings.sshKeys.dialog.creating") : newSSHKeyPrivate ? t("settings.sshKeys.dialog.importKey") : t("settings.sshKeys.dialog.generateKey")}
                    </Button>
                  </div>
                </>
              )}
            </div>
          </div>
        )}
      </div>
    </div>
  );
}

function BillingSettings({ t }: { t: TranslationFn }) {
  const [loading, setLoading] = useState(true);
  const [overview, setOverview] = useState<BillingOverview | null>(null);
  const [plans, setPlans] = useState<SubscriptionPlan[]>([]);
  const [error, setError] = useState<string | null>(null);
  const [showPlansDialog, setShowPlansDialog] = useState(false);
  const [upgrading, setUpgrading] = useState(false);

  const loadBillingData = useCallback(async () => {
    setLoading(true);
    setError(null);
    try {
      const [overviewRes, plansRes] = await Promise.all([
        billingApi.getOverview().catch(() => null),
        billingApi.listPlans().catch(() => ({ plans: [] })),
      ]);
      if (overviewRes?.overview) {
        setOverview(overviewRes.overview);
      }
      setPlans(plansRes.plans || []);
    } catch (err) {
      setError("Failed to load billing data");
      console.error("Error loading billing data:", err);
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    loadBillingData();
  }, [loadBillingData]);

  const handleUpgrade = async (planName: string) => {
    setUpgrading(true);
    try {
      if (overview) {
        await billingApi.updateSubscription(planName);
      } else {
        await billingApi.createSubscription(planName);
      }
      setShowPlansDialog(false);
      await loadBillingData();
    } catch (err) {
      console.error("Failed to upgrade:", err);
      setError("Failed to upgrade plan");
    } finally {
      setUpgrading(false);
    }
  };

  // Calculate usage percentages
  const getUsagePercent = (current: number, max: number): number => {
    if (max === -1) return 0; // Unlimited
    if (max === 0) return 100;
    return Math.min(100, (current / max) * 100);
  };

  const formatLimit = (value: number): string => {
    return value === -1 ? t("settings.billingPage.unlimited") : String(value);
  };

  if (loading) {
    return (
      <div className="space-y-6">
        <div className="border border-border rounded-lg p-6 animate-pulse">
          <div className="h-6 bg-muted rounded w-32 mb-4"></div>
          <div className="h-8 bg-muted rounded w-48 mb-2"></div>
          <div className="h-4 bg-muted rounded w-64"></div>
        </div>
      </div>
    );
  }

  if (error && !overview) {
    return (
      <div className="space-y-6">
        <div className="border border-border rounded-lg p-6">
          <p className="text-destructive">{error}</p>
          <Button variant="outline" className="mt-4" onClick={loadBillingData}>
            {t("settings.billingPage.retry")}
          </Button>
        </div>
      </div>
    );
  }

  // If no subscription exists, show setup prompt
  if (!overview) {
    return (
      <div className="space-y-6">
        <div className="border border-border rounded-lg p-6 text-center">
          <h2 className="text-lg font-semibold mb-4">{t("settings.billingPage.noSubscription")}</h2>
          <p className="text-muted-foreground mb-6">
            {t("settings.billingPage.choosePlan")}
          </p>
          <Button onClick={() => setShowPlansDialog(true)}>{t("settings.billingPage.selectPlan")}</Button>
        </div>

        {/* Plans Dialog */}
        {showPlansDialog && (
          <PlansDialog
            plans={plans}
            currentPlan={null}
            onSelect={handleUpgrade}
            onClose={() => setShowPlansDialog(false)}
            loading={upgrading}
            t={t}
          />
        )}
      </div>
    );
  }

  const { plan, usage, status, billing_cycle, current_period_end } = overview;

  return (
    <div className="space-y-6">
      {/* Current Plan */}
      <div className="border border-border rounded-lg p-6">
        <h2 className="text-lg font-semibold mb-4">{t("settings.billingPage.currentPlan")}</h2>
        <div className="flex items-center justify-between">
          <div>
            <div className="flex items-center gap-3">
              <h3 className="text-2xl font-bold">{plan?.display_name || plan?.name || t("settings.billingPage.plansDialog.free")}</h3>
              <span className={`text-xs px-2 py-0.5 rounded ${
                status === "active" ? "bg-green-100 text-green-800" :
                status === "past_due" ? "bg-yellow-100 text-yellow-800" :
                "bg-red-100 text-red-800"
              }`}>
                {status.charAt(0).toUpperCase() + status.slice(1)}
              </span>
            </div>
            <p className="text-muted-foreground">
              {billing_cycle === "yearly" ? t("settings.billingPage.yearly") : t("settings.billingPage.monthly")} billing
              {current_period_end && (
                <> · {t("settings.billingPage.renews")} {new Date(current_period_end).toLocaleDateString()}</>
              )}
            </p>
            {plan?.price_per_seat_monthly > 0 && (
              <p className="text-sm text-muted-foreground mt-1">
                ${plan.price_per_seat_monthly}/seat/month
              </p>
            )}
          </div>
          <Button onClick={() => setShowPlansDialog(true)}>
            {plan?.name === "free" ? t("settings.billingPage.upgrade") : t("settings.billingPage.changePlan")}
          </Button>
        </div>
      </div>

      {/* Usage */}
      <div className="border border-border rounded-lg p-6">
        <h2 className="text-lg font-semibold mb-4">{t("settings.billingPage.usage")}</h2>
        <div className="space-y-4">
          {/* Pod Minutes */}
          <div>
            <div className="flex items-center justify-between mb-2">
              <span className="text-sm">{t("settings.billingPage.podMinutes")}</span>
              <span className="text-sm font-medium">
                {Math.round(usage.pod_minutes)} / {formatLimit(usage.included_pod_minutes)}
              </span>
            </div>
            <div className="w-full bg-muted rounded-full h-2">
              <div
                className={`h-2 rounded-full ${
                  getUsagePercent(usage.pod_minutes, usage.included_pod_minutes) > 90
                    ? "bg-destructive"
                    : "bg-primary"
                }`}
                style={{ width: `${getUsagePercent(usage.pod_minutes, usage.included_pod_minutes)}%` }}
              ></div>
            </div>
          </div>

          {/* Users */}
          <div>
            <div className="flex items-center justify-between mb-2">
              <span className="text-sm">{t("settings.billingPage.teamMembers")}</span>
              <span className="text-sm font-medium">
                {usage.users} / {formatLimit(usage.max_users)}
              </span>
            </div>
            <div className="w-full bg-muted rounded-full h-2">
              <div
                className={`h-2 rounded-full ${
                  getUsagePercent(usage.users, usage.max_users) > 90 ? "bg-destructive" : "bg-primary"
                }`}
                style={{ width: `${getUsagePercent(usage.users, usage.max_users)}%` }}
              ></div>
            </div>
          </div>

          {/* Runners */}
          <div>
            <div className="flex items-center justify-between mb-2">
              <span className="text-sm">Runners</span>
              <span className="text-sm font-medium">
                {usage.runners} / {formatLimit(usage.max_runners)}
              </span>
            </div>
            <div className="w-full bg-muted rounded-full h-2">
              <div
                className={`h-2 rounded-full ${
                  getUsagePercent(usage.runners, usage.max_runners) > 90 ? "bg-destructive" : "bg-primary"
                }`}
                style={{ width: `${getUsagePercent(usage.runners, usage.max_runners)}%` }}
              ></div>
            </div>
          </div>

          {/* Repositories */}
          <div>
            <div className="flex items-center justify-between mb-2">
              <span className="text-sm">{t("settings.billingPage.repositories")}</span>
              <span className="text-sm font-medium">
                {usage.repositories} / {formatLimit(usage.max_repositories)}
              </span>
            </div>
            <div className="w-full bg-muted rounded-full h-2">
              <div
                className={`h-2 rounded-full ${
                  getUsagePercent(usage.repositories, usage.max_repositories) > 90
                    ? "bg-destructive"
                    : "bg-primary"
                }`}
                style={{ width: `${getUsagePercent(usage.repositories, usage.max_repositories)}%` }}
              ></div>
            </div>
          </div>
        </div>
      </div>

      {/* Promo Code */}
      <div className="border border-border rounded-lg p-6">
        <h2 className="text-lg font-semibold mb-2">{t("settings.billingPage.promoCode.title")}</h2>
        <p className="text-sm text-muted-foreground mb-4">
          {t("settings.billingPage.promoCode.description")}
        </p>
        <PromoCodeInput
          onRedeemSuccess={(response: RedeemPromoCodeResponse) => {
            // Reload billing data after successful redemption
            loadBillingData();
          }}
          t={(key: string) => t(`settings.billingPage.promoCode.${key}`)}
        />
      </div>

      {/* Plans Dialog */}
      {showPlansDialog && (
        <PlansDialog
          plans={plans}
          currentPlan={plan?.name || null}
          onSelect={handleUpgrade}
          onClose={() => setShowPlansDialog(false)}
          loading={upgrading}
          t={t}
        />
      )}
    </div>
  );
}

// Plans selection dialog component
function PlansDialog({
  plans,
  currentPlan,
  onSelect,
  onClose,
  loading,
  t,
}: {
  plans: SubscriptionPlan[];
  currentPlan: string | null;
  onSelect: (planName: string) => void;
  onClose: () => void;
  loading: boolean;
  t: TranslationFn;
}) {
  const formatLimit = (value: number): string => {
    return value === -1 ? t("settings.billingPage.unlimited") : String(value);
  };

  return (
    <div className="fixed inset-0 bg-black/50 flex items-center justify-center z-50">
      <div className="bg-background border border-border rounded-lg p-6 w-full max-w-4xl max-h-[90vh] overflow-y-auto">
        <div className="flex items-center justify-between mb-6">
          <h3 className="text-lg font-semibold">{t("settings.billingPage.plansDialog.title")}</h3>
          <button onClick={onClose} className="text-muted-foreground hover:text-foreground">
            ✕
          </button>
        </div>

        {plans.length === 0 ? (
          <p className="text-center text-muted-foreground py-8">{t("settings.billingPage.plansDialog.noPlans")}</p>
        ) : (
          <div className="grid grid-cols-1 md:grid-cols-3 gap-4">
            {plans.map((plan) => {
              const isCurrent = plan.name === currentPlan;
              return (
                <div
                  key={plan.id}
                  className={`border rounded-lg p-6 ${
                    isCurrent ? "border-primary bg-primary/5" : "border-border"
                  }`}
                >
                  <div className="mb-4">
                    <h4 className="text-xl font-bold">{plan.display_name}</h4>
                    {plan.price_per_seat_monthly > 0 ? (
                      <p className="text-2xl font-bold mt-2">
                        ${plan.price_per_seat_monthly}
                        <span className="text-sm font-normal text-muted-foreground">/seat/month</span>
                      </p>
                    ) : (
                      <p className="text-2xl font-bold mt-2">{t("settings.billingPage.plansDialog.free")}</p>
                    )}
                  </div>

                  <ul className="space-y-2 mb-6 text-sm">
                    <li className="flex items-center gap-2">
                      <span className="text-green-500">✓</span>
                      {formatLimit(plan.included_pod_minutes)} {t("settings.billingPage.plansDialog.podMinutes")}
                    </li>
                    <li className="flex items-center gap-2">
                      <span className="text-green-500">✓</span>
                      {formatLimit(plan.max_users)} {t("settings.billingPage.plansDialog.teamMembers")}
                    </li>
                    <li className="flex items-center gap-2">
                      <span className="text-green-500">✓</span>
                      {formatLimit(plan.max_runners)} {t("settings.billingPage.plansDialog.runners")}
                    </li>
                    <li className="flex items-center gap-2">
                      <span className="text-green-500">✓</span>
                      {formatLimit(plan.max_repositories)} {t("settings.billingPage.plansDialog.repositories")}
                    </li>
                  </ul>

                  <Button
                    className="w-full"
                    variant={isCurrent ? "outline" : "default"}
                    disabled={isCurrent || loading}
                    onClick={() => onSelect(plan.name)}
                  >
                    {loading ? t("settings.billingPage.plansDialog.processing") : isCurrent ? t("settings.billingPage.plansDialog.currentPlan") : t("settings.billingPage.plansDialog.select")}
                  </Button>
                </div>
              );
            })}
          </div>
        )}
      </div>
    </div>
  );
}

// ===== Runners Settings =====

function RunnersSettings({ t }: { t: TranslationFn }) {
  const {
    runners,
    tokens,
    loading,
    error,
    fetchRunners,
    fetchTokens,
    updateRunner,
    deleteRunner,
    regenerateAuthToken,
    createToken,
    revokeToken,
    clearError,
  } = useRunnerStore();

  const [editingRunner, setEditingRunner] = useState<Runner | null>(null);
  const [showTokenDialog, setShowTokenDialog] = useState(false);

  useEffect(() => {
    fetchRunners();
    fetchTokens();
  }, [fetchRunners, fetchTokens]);

  return (
    <div className="space-y-6">
      {error && (
        <div className="bg-destructive/10 border border-destructive text-destructive px-4 py-3 rounded-lg flex items-center justify-between">
          <span>{error}</span>
          <button onClick={clearError} className="text-sm underline">
            {t("settings.members.dismiss")}
          </button>
        </div>
      )}

      {/* Registration Tokens */}
      <TokensPanel
        tokens={tokens}
        loading={loading}
        onCreateToken={createToken}
        onRevokeToken={revokeToken}
        showDialog={showTokenDialog}
        onShowDialog={setShowTokenDialog}
        t={t}
      />

      {/* Runners List */}
      <RunnersPanel
        runners={runners}
        loading={loading}
        onEdit={setEditingRunner}
        onDelete={deleteRunner}
        onRegenerateToken={regenerateAuthToken}
        t={t}
      />

      {/* Edit Runner Dialog */}
      {editingRunner && (
        <EditRunnerDialog
          runner={editingRunner}
          onClose={() => setEditingRunner(null)}
          onSave={async (id, data) => {
            await updateRunner(id, data);
            setEditingRunner(null);
          }}
          t={t}
        />
      )}
    </div>
  );
}

// TokensPanel Component
function TokensPanel({
  tokens,
  loading,
  onCreateToken,
  onRevokeToken,
  showDialog,
  onShowDialog,
  t,
}: {
  tokens: RegistrationToken[];
  loading: boolean;
  onCreateToken: (description?: string, maxUses?: number, expiresAt?: string) => Promise<string>;
  onRevokeToken: (id: number) => Promise<void>;
  showDialog: boolean;
  onShowDialog: (show: boolean) => void;
  t: TranslationFn;
}) {
  const [newTokenDescription, setNewTokenDescription] = useState("");
  const [newTokenMaxUses, setNewTokenMaxUses] = useState<string>("");
  const [newTokenExpires, setNewTokenExpires] = useState<string>("");
  const [createdToken, setCreatedToken] = useState<string | null>(null);
  const [creating, setCreating] = useState(false);

  const handleCreateToken = async () => {
    setCreating(true);
    try {
      const maxUses = newTokenMaxUses ? parseInt(newTokenMaxUses, 10) : undefined;
      const expiresAt = newTokenExpires || undefined;
      const token = await onCreateToken(newTokenDescription || undefined, maxUses, expiresAt);
      setCreatedToken(token);
      setNewTokenDescription("");
      setNewTokenMaxUses("");
      setNewTokenExpires("");
    } catch (err) {
      console.error("Failed to create token:", err);
    } finally {
      setCreating(false);
    }
  };

  const handleCloseDialog = () => {
    onShowDialog(false);
    setCreatedToken(null);
    setNewTokenDescription("");
    setNewTokenMaxUses("");
    setNewTokenExpires("");
  };

  const copyToClipboard = (text: string) => {
    navigator.clipboard.writeText(text);
  };

  const formatDate = (dateStr: string) => {
    return new Date(dateStr).toLocaleDateString();
  };

  return (
    <div className="border border-border rounded-lg p-6">
      <div className="flex items-center justify-between mb-4">
        <div>
          <h2 className="text-lg font-semibold">{t("settings.tokensSection.title")}</h2>
          <p className="text-sm text-muted-foreground">
            {t("settings.tokensSection.description")}
          </p>
        </div>
        <Button onClick={() => onShowDialog(true)}>{t("settings.tokensSection.createToken")}</Button>
      </div>

      {loading ? (
        <div className="text-center py-4 text-muted-foreground">{t("settings.tokensSection.loading")}</div>
      ) : tokens.length === 0 ? (
        <div className="text-center py-8 text-muted-foreground">
          {t("settings.tokensSection.noTokens")}
        </div>
      ) : (
        <div className="space-y-3">
          {tokens.map((token) => (
            <div
              key={token.id}
              className={`flex items-center justify-between p-4 border rounded-lg ${
                token.is_active ? "border-border" : "border-border bg-muted/50 opacity-60"
              }`}
            >
              <div className="flex-1">
                <div className="flex items-center gap-2">
                  <span className="font-medium">
                    {token.description || `Token #${token.id}`}
                  </span>
                  {!token.is_active && (
                    <span className="text-xs bg-muted px-2 py-0.5 rounded">{t("settings.tokensSection.revoked")}</span>
                  )}
                </div>
                <div className="text-sm text-muted-foreground mt-1 space-x-4">
                  <span>{t("settings.tokensSection.uses")} {token.used_count}{token.max_uses ? ` / ${token.max_uses}` : ""}</span>
                  <span>{t("settings.tokensSection.created")} {formatDate(token.created_at)}</span>
                  {token.expires_at && (
                    <span>{t("settings.tokensSection.expires")} {formatDate(token.expires_at)}</span>
                  )}
                </div>
              </div>
              {token.is_active && (
                <Button
                  variant="ghost"
                  size="sm"
                  className="text-destructive hover:text-destructive"
                  onClick={() => onRevokeToken(token.id)}
                >
                  {t("settings.tokensSection.revoke")}
                </Button>
              )}
            </div>
          ))}
        </div>
      )}

      {/* Create Token Dialog */}
      {showDialog && (
        <div className="fixed inset-0 bg-black/50 flex items-center justify-center z-50">
          <div className="bg-background border border-border rounded-lg p-6 w-full max-w-md">
            {createdToken ? (
              <>
                <h3 className="text-lg font-semibold mb-4">{t("settings.tokensSection.dialog.createdTitle")}</h3>
                <p className="text-sm text-muted-foreground mb-4">
                  {t("settings.tokensSection.dialog.createdHint")}
                </p>
                <div className="bg-muted p-3 rounded-lg mb-4 flex items-center justify-between">
                  <code className="text-sm break-all">{createdToken}</code>
                  <Button
                    variant="ghost"
                    size="sm"
                    onClick={() => copyToClipboard(createdToken)}
                  >
                    {t("settings.tokensSection.dialog.copy")}
                  </Button>
                </div>
                <Button className="w-full" onClick={handleCloseDialog}>
                  {t("settings.tokensSection.dialog.done")}
                </Button>
              </>
            ) : (
              <>
                <h3 className="text-lg font-semibold mb-4">{t("settings.tokensSection.dialog.createTitle")}</h3>
                <div className="space-y-4">
                  <div>
                    <label className="block text-sm font-medium mb-2">
                      {t("settings.tokensSection.dialog.descriptionLabel")}
                    </label>
                    <Input
                      value={newTokenDescription}
                      onChange={(e) => setNewTokenDescription(e.target.value)}
                      placeholder={t("settings.tokensSection.dialog.descriptionPlaceholder")}
                    />
                  </div>
                  <div>
                    <label className="block text-sm font-medium mb-2">
                      {t("settings.tokensSection.dialog.maxUsesLabel")}
                    </label>
                    <Input
                      type="number"
                      value={newTokenMaxUses}
                      onChange={(e) => setNewTokenMaxUses(e.target.value)}
                      placeholder={t("settings.tokensSection.dialog.maxUsesPlaceholder")}
                      min="1"
                    />
                  </div>
                  <div>
                    <label className="block text-sm font-medium mb-2">
                      {t("settings.tokensSection.dialog.expiresLabel")}
                    </label>
                    <Input
                      type="datetime-local"
                      value={newTokenExpires}
                      onChange={(e) => setNewTokenExpires(e.target.value)}
                    />
                  </div>
                </div>
                <div className="flex gap-3 mt-6">
                  <Button variant="outline" className="flex-1" onClick={handleCloseDialog}>
                    {t("settings.tokensSection.dialog.cancel")}
                  </Button>
                  <Button className="flex-1" onClick={handleCreateToken} disabled={creating}>
                    {creating ? t("settings.tokensSection.dialog.creating") : t("settings.tokensSection.dialog.createToken")}
                  </Button>
                </div>
              </>
            )}
          </div>
        </div>
      )}
    </div>
  );
}

// RunnersPanel Component
function RunnersPanel({
  runners,
  loading,
  onEdit,
  onDelete,
  onRegenerateToken,
  t,
}: {
  runners: Runner[];
  loading: boolean;
  onEdit: (runner: Runner) => void;
  onDelete: (id: number) => Promise<void>;
  onRegenerateToken: (id: number) => Promise<string>;
  t: TranslationFn;
}) {
  const [confirmDelete, setConfirmDelete] = useState<number | null>(null);
  const [regeneratedToken, setRegeneratedToken] = useState<{ id: number; token: string } | null>(null);

  const handleDelete = async (id: number) => {
    try {
      await onDelete(id);
      setConfirmDelete(null);
    } catch (err) {
      console.error("Failed to delete runner:", err);
    }
  };

  const handleRegenerateToken = async (id: number) => {
    try {
      const token = await onRegenerateToken(id);
      setRegeneratedToken({ id, token });
    } catch (err) {
      console.error("Failed to regenerate token:", err);
    }
  };

  const copyToClipboard = (text: string) => {
    navigator.clipboard.writeText(text);
  };

  const formatLastSeen = (dateString?: string) => {
    if (!dateString) return "Never";
    const date = new Date(dateString);
    const now = new Date();
    const diffMs = now.getTime() - date.getTime();
    const diffSec = Math.floor(diffMs / 1000);

    if (diffSec < 60) return t("settings.runnersSection.justNow");
    if (diffSec < 3600) return `${Math.floor(diffSec / 60)}m ago`;
    if (diffSec < 86400) return `${Math.floor(diffSec / 3600)}h ago`;
    return date.toLocaleDateString();
  };

  return (
    <div className="border border-border rounded-lg p-6">
      <div className="mb-4">
        <h2 className="text-lg font-semibold">{t("settings.runnersSection.title")}</h2>
        <p className="text-sm text-muted-foreground">
          {t("settings.runnersSection.description")}
        </p>
      </div>

      {loading ? (
        <div className="text-center py-4 text-muted-foreground">{t("settings.runnersSection.loading")}</div>
      ) : runners.length === 0 ? (
        <div className="text-center py-8 text-muted-foreground">
          {t("settings.runnersSection.noRunners")}
        </div>
      ) : (
        <div className="space-y-3">
          {runners.map((runner) => {
            const statusInfo = getRunnerStatusInfo(runner.status as "online" | "offline" | "maintenance" | "busy");
            return (
              <div
                key={runner.id}
                className={`p-4 border rounded-lg ${
                  runner.is_enabled ? "border-border" : "border-border bg-muted/50"
                }`}
              >
                <div className="flex items-start justify-between">
                  <div className="flex-1">
                    <div className="flex items-center gap-2">
                      <span className="font-medium">{runner.node_id}</span>
                      <span
                        className={`inline-flex items-center gap-1 px-2 py-0.5 rounded-full text-xs font-medium ${statusInfo?.color}`}
                      >
                        <span className={`w-1.5 h-1.5 rounded-full ${statusInfo?.dotColor}`} />
                        {statusInfo?.label}
                      </span>
                      {!runner.is_enabled && (
                        <span className="text-xs bg-yellow-100 text-yellow-800 px-2 py-0.5 rounded">
                          {t("settings.runnersSection.disabled")}
                        </span>
                      )}
                    </div>
                    {runner.description && (
                      <p className="text-sm text-muted-foreground mt-1">
                        {runner.description}
                      </p>
                    )}
                    <div className="flex items-center gap-4 text-sm text-muted-foreground mt-2">
                      <span>
                        {t("settings.runnersSection.pods")} {runner.current_pods} / {runner.max_concurrent_pods}
                      </span>
                      {runner.runner_version && <span>v{runner.runner_version}</span>}
                      <span>{t("settings.runnersSection.lastSeen")} {formatLastSeen(runner.last_heartbeat)}</span>
                    </div>
                  </div>
                  <div className="flex items-center gap-2">
                    <Button variant="outline" size="sm" onClick={() => onEdit(runner)}>
                      {t("settings.runnersSection.edit")}
                    </Button>
                    <Button
                      variant="outline"
                      size="sm"
                      onClick={() => handleRegenerateToken(runner.id)}
                    >
                      {t("settings.runnersSection.regenerateToken")}
                    </Button>
                    <Button
                      variant="ghost"
                      size="sm"
                      className="text-destructive hover:text-destructive"
                      onClick={() => setConfirmDelete(runner.id)}
                    >
                      {t("settings.runnersSection.delete")}
                    </Button>
                  </div>
                </div>
              </div>
            );
          })}
        </div>
      )}

      {/* Confirm Delete Dialog */}
      {confirmDelete !== null && (
        <div className="fixed inset-0 bg-black/50 flex items-center justify-center z-50">
          <div className="bg-background border border-border rounded-lg p-6 w-full max-w-sm">
            <h3 className="text-lg font-semibold mb-2">{t("settings.runnersSection.deleteDialog.title")}</h3>
            <p className="text-muted-foreground mb-4">
              {t("settings.runnersSection.deleteDialog.description")}
            </p>
            <div className="flex gap-3">
              <Button variant="outline" className="flex-1" onClick={() => setConfirmDelete(null)}>
                {t("settings.runnersSection.deleteDialog.cancel")}
              </Button>
              <Button
                variant="destructive"
                className="flex-1"
                onClick={() => handleDelete(confirmDelete)}
              >
                {t("settings.runnersSection.deleteDialog.delete")}
              </Button>
            </div>
          </div>
        </div>
      )}

      {/* Regenerated Token Dialog */}
      {regeneratedToken && (
        <div className="fixed inset-0 bg-black/50 flex items-center justify-center z-50">
          <div className="bg-background border border-border rounded-lg p-6 w-full max-w-md">
            <h3 className="text-lg font-semibold mb-4">{t("settings.runnersSection.tokenDialog.title")}</h3>
            <p className="text-sm text-muted-foreground mb-4">
              {t("settings.runnersSection.tokenDialog.description")}
            </p>
            <div className="bg-muted p-3 rounded-lg mb-4 flex items-center justify-between">
              <code className="text-sm break-all">{regeneratedToken.token}</code>
              <Button
                variant="ghost"
                size="sm"
                onClick={() => copyToClipboard(regeneratedToken.token)}
              >
                {t("settings.runnersSection.tokenDialog.copy")}
              </Button>
            </div>
            <Button className="w-full" onClick={() => setRegeneratedToken(null)}>
              {t("settings.runnersSection.tokenDialog.done")}
            </Button>
          </div>
        </div>
      )}
    </div>
  );
}

// Edit Runner Dialog Component
function EditRunnerDialog({
  runner,
  onClose,
  onSave,
  t,
}: {
  runner: Runner;
  onClose: () => void;
  onSave: (id: number, data: { description?: string; max_concurrent_pods?: number; is_enabled?: boolean }) => Promise<void>;
  t: TranslationFn;
}) {
  const [description, setDescription] = useState(runner.description || "");
  const [maxPods, setMaxPods] = useState(runner.max_concurrent_pods.toString());
  const [isEnabled, setIsEnabled] = useState(runner.is_enabled);
  const [saving, setSaving] = useState(false);

  const handleSave = async () => {
    setSaving(true);
    try {
      await onSave(runner.id, {
        description: description || undefined,
        max_concurrent_pods: parseInt(maxPods, 10),
        is_enabled: isEnabled,
      });
    } catch (err) {
      console.error("Failed to save runner:", err);
    } finally {
      setSaving(false);
    }
  };

  return (
    <div className="fixed inset-0 bg-black/50 flex items-center justify-center z-50">
      <div className="bg-background border border-border rounded-lg p-6 w-full max-w-md">
        <h3 className="text-lg font-semibold mb-4">{t("settings.runnersSection.editDialog.title")}</h3>
        <div className="space-y-4">
          <div>
            <label className="block text-sm font-medium mb-2">{t("settings.runnersSection.editDialog.nodeIdLabel")}</label>
            <Input value={runner.node_id} disabled />
          </div>
          <div>
            <label className="block text-sm font-medium mb-2">{t("settings.runnersSection.editDialog.descriptionLabel")}</label>
            <Input
              value={description}
              onChange={(e) => setDescription(e.target.value)}
              placeholder={t("settings.runnersSection.editDialog.descriptionPlaceholder")}
            />
          </div>
          <div>
            <label className="block text-sm font-medium mb-2">
              {t("settings.runnersSection.editDialog.maxPodsLabel")}
            </label>
            <Input
              type="number"
              value={maxPods}
              onChange={(e) => setMaxPods(e.target.value)}
              min="1"
            />
          </div>
          <div className="flex items-center justify-between">
            <label className="text-sm font-medium">{t("settings.runnersSection.editDialog.enabledLabel")}</label>
            <label className="relative inline-flex items-center cursor-pointer">
              <input
                type="checkbox"
                className="sr-only peer"
                checked={isEnabled}
                onChange={(e) => setIsEnabled(e.target.checked)}
              />
              <div className="w-11 h-6 bg-gray-200 peer-focus:outline-none rounded-full peer peer-checked:after:translate-x-full peer-checked:after:border-white after:content-[''] after:absolute after:top-[2px] after:left-[2px] after:bg-white after:border-gray-300 after:border after:rounded-full after:h-5 after:w-5 after:transition-all peer-checked:bg-primary"></div>
            </label>
          </div>
        </div>
        <div className="flex gap-3 mt-6">
          <Button variant="outline" className="flex-1" onClick={onClose}>
            {t("settings.runnersSection.editDialog.cancel")}
          </Button>
          <Button className="flex-1" onClick={handleSave} disabled={saving}>
            {saving ? t("settings.runnersSection.editDialog.saving") : t("settings.runnersSection.editDialog.saveChanges")}
          </Button>
        </div>
      </div>
    </div>
  );
}

// ===== Notifications Settings =====

function NotificationsSettings({ t }: { t: TranslationFn }) {
  return (
    <div className="space-y-6">
      <div className="border border-border rounded-lg p-6">
        <h2 className="text-lg font-semibold mb-4">{t("settings.notifications.title")}</h2>
        <p className="text-sm text-muted-foreground mb-6">
          {t("settings.notifications.description")}
        </p>
        <NotificationSettings />
      </div>
    </div>
  );
}
