import { request, orgPath } from "./base";

// Agent type interface
interface AgentTypeResponse {
  id: number;
  slug: string;
  name: string;
  description?: string;
  launch_command?: string;
  is_builtin: boolean;
  is_active: boolean;
}

// Agents API
export const agentApi = {
  listTypes: async () => {
    const response = await request<{
      builtin_types: AgentTypeResponse[];
      custom_types: AgentTypeResponse[];
    }>(orgPath("/agents/types"));
    // Combine builtin and custom types for frontend compatibility
    return {
      agent_types: [...(response.builtin_types || []), ...(response.custom_types || [])],
    };
  },

  getConfig: () =>
    request<{ config: unknown }>(orgPath("/agents/config")),

  updateConfig: (data: unknown) =>
    request<{ message: string }>(orgPath("/agents/config"), {
      method: "PUT",
      body: data,
    }),

  listCredentials: () =>
    request<{ credentials: unknown[] }>(orgPath("/agents/credentials")),

  updateCredentials: (agentType: string, credentials: Record<string, string>) =>
    request<{ message: string }>(`${orgPath("/agents/credentials")}/${agentType}`, {
      method: "PUT",
      body: { credentials },
    }),
};
