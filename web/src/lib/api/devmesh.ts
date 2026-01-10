import { request, orgPath } from "./base";

// DevMesh types
export interface DevMeshNodeData {
  pod_key: string;
  status: string;
  agent_status: string;
  model?: string;
  ticket_id?: number;
  repository_id?: number;
  created_by_id: number;
  runner_id: number;
  started_at?: string;
  position?: { x: number; y: number };
}

export interface DevMeshEdgeData {
  id: number;
  source: string;
  target: string;
  granted_scopes: string[];
  pending_scopes?: string[];
  status: string;
}

export interface ChannelInfoData {
  id: number;
  name: string;
  description?: string;
  pod_keys: string[];
  message_count: number;
  is_archived: boolean;
}

export interface DevMeshTopologyData {
  nodes: DevMeshNodeData[];
  edges: DevMeshEdgeData[];
  channels: ChannelInfoData[];
}

// DevMesh API
export const devmeshApi = {
  // Get topology
  getTopology: () => {
    return request<{ topology: DevMeshTopologyData }>(orgPath("/devmesh/topology"));
  },

  // For channel operations, use channelApi.joinPod() and channelApi.leavePod()
};
