import { request, orgPath } from "./base";

// Git Provider types
export interface GitProviderData {
  id: number;
  organization_id: number;
  provider_type: "github" | "gitlab" | "gitee" | "ssh";
  name: string;
  base_url: string;
  ssh_key_id?: number;
  is_default: boolean;
  is_active: boolean;
  created_at: string;
  updated_at: string;
}

// Git Provider API
export const gitProviderApi = {
  list: () =>
    request<{ git_providers: GitProviderData[] }>(orgPath("/git-providers")),

  get: (id: number) =>
    request<{ git_provider: GitProviderData }>(`${orgPath("/git-providers")}/${id}`),

  create: (data: {
    provider_type: string;
    name: string;
    base_url: string;
    client_id?: string;
    client_secret?: string;
    bot_token?: string;
    ssh_key_id?: number;
    is_default?: boolean;
  }) =>
    request<{ git_provider: GitProviderData }>(orgPath("/git-providers"), {
      method: "POST",
      body: data,
    }),

  update: (id: number, data: {
    name?: string;
    base_url?: string;
    client_id?: string;
    client_secret?: string;
    bot_token?: string;
    ssh_key_id?: number;
    is_default?: boolean;
    is_active?: boolean;
  }) =>
    request<{ git_provider: GitProviderData }>(`${orgPath("/git-providers")}/${id}`, {
      method: "PUT",
      body: data,
    }),

  delete: (id: number) =>
    request<{ message: string }>(`${orgPath("/git-providers")}/${id}`, {
      method: "DELETE",
    }),

  testConnection: (id: number) =>
    request<{ success: boolean; message: string }>(`${orgPath("/git-providers")}/${id}/test`, {
      method: "POST",
    }),
};
