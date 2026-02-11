import { useState, useEffect, useMemo } from "react";
import {
  runnerApi,
  agentApi,
  repositoryApi,
  RunnerData,
  AgentTypeData,
  RepositoryData,
} from "@/lib/api";

export interface PodCreationData {
  runners: RunnerData[];
  agentTypes: AgentTypeData[];
  repositories: RepositoryData[];
  loading: boolean;
  error: string | null;
  // Runner selection state
  selectedRunner: RunnerData | null;
  setSelectedRunnerId: (id: number | null) => void;
  // Agent types filtered by selected runner's available agents
  availableAgentTypes: AgentTypeData[];
}

/**
 * Hook to load data required for pod creation (runners, agents, repositories)
 * Agent types are filtered based on the selected runner's available agents
 * Only loads when enabled is true (e.g., when modal is open)
 */
export function usePodCreationData(enabled: boolean): PodCreationData {
  const [runners, setRunners] = useState<RunnerData[]>([]);
  const [agentTypes, setAgentTypes] = useState<AgentTypeData[]>([]);
  const [repositories, setRepositories] = useState<RepositoryData[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [selectedRunnerId, setSelectedRunnerId] = useState<number | null>(null);

  // Load runners, agents, and repositories
  useEffect(() => {
    if (!enabled) return;

    let cancelled = false;

    const loadData = async () => {
      setLoading(true);
      setError(null);
      try {
        const [runnersRes, agentsRes, reposRes] = await Promise.allSettled([
          runnerApi.list(),
          agentApi.listTypes(),
          repositoryApi.list(),
        ]);

        if (cancelled) return;

        if (runnersRes.status === "fulfilled") {
          // Only online runners
          const allRunners = runnersRes.value.runners || [];
          const onlineRunners = allRunners.filter(r => r.status === "online");
          console.log("[usePodCreationData] Runners loaded:", allRunners.length, "total,", onlineRunners.length, "online");
          setRunners(onlineRunners);
        }
        if (agentsRes.status === "fulfilled") {
          const agents = agentsRes.value.agent_types || [];
          console.log("[usePodCreationData] Agent types loaded:", agents.length, agents.map(a => ({ id: a.id, slug: a.slug })));
          setAgentTypes(agents);
        }
        if (reposRes.status === "fulfilled") {
          setRepositories(reposRes.value.repositories || []);
        }
      } catch (err) {
        if (cancelled) return;
        const message = err instanceof Error ? err.message : "Failed to load data";
        setError(message);
        console.error("Failed to load pod creation data:", err);
      } finally {
        if (!cancelled) {
          setLoading(false);
        }
      }
    };

    loadData();

    return () => {
      cancelled = true;
    };
  }, [enabled]);

  // Reset selected runner when modal closes
  useEffect(() => {
    if (!enabled) {
      setSelectedRunnerId(null);
    }
  }, [enabled]);

  // Get selected runner object
  const selectedRunner = useMemo(() => {
    if (!selectedRunnerId) return null;
    return runners.find(r => r.id === selectedRunnerId) || null;
  }, [runners, selectedRunnerId]);

  // Filter agent types based on selected runner's available agents
  // When no runner is manually selected: union of all online runners' available agents
  // When runner is manually selected: filter by that runner's available agents
  const availableAgentTypes = useMemo((): AgentTypeData[] => {
    if (selectedRunner?.available_agents?.length) {
      return agentTypes.filter(agent => selectedRunner.available_agents!.includes(agent.slug));
    }

    // No runner selected: show union of all online runners' available agents
    const allSlugs = new Set(runners.flatMap(r => r.available_agents || []));
    if (allSlugs.size === 0) return [];
    return agentTypes.filter(agent => allSlugs.has(agent.slug));
  }, [selectedRunner, runners, agentTypes]);

  return {
    runners,
    agentTypes,
    repositories,
    loading,
    error,
    selectedRunner,
    setSelectedRunnerId,
    availableAgentTypes,
  };
}
