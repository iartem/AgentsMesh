import { create } from "zustand";

export interface OrganizationMember {
  id: number;
  user_id: number;
  username: string;
  email: string;
  name?: string;
  avatar_url?: string;
  role: "owner" | "admin" | "member";
  joined_at: string;
}

export interface Organization {
  id: number;
  name: string;
  slug: string;
  logo_url?: string;
  subscription_plan: string;
  subscription_status: string;
  created_at: string;
  updated_at: string;
}

interface OrganizationState {
  organizations: Organization[];
  currentOrganization: Organization | null;
  members: OrganizationMember[];
  isLoading: boolean;
  error: string | null;

  // Actions
  setOrganizations: (orgs: Organization[]) => void;
  setCurrentOrganization: (org: Organization | null) => void;
  addOrganization: (org: Organization) => void;
  updateOrganization: (id: number, updates: Partial<Organization>) => void;
  removeOrganization: (id: number) => void;
  setMembers: (members: OrganizationMember[]) => void;
  addMember: (member: OrganizationMember) => void;
  updateMember: (userId: number, updates: Partial<OrganizationMember>) => void;
  removeMember: (userId: number) => void;
  setLoading: (loading: boolean) => void;
  setError: (error: string | null) => void;
  reset: () => void;
}

const initialState = {
  organizations: [],
  currentOrganization: null,
  members: [],
  isLoading: false,
  error: null,
};

export const useOrganizationStore = create<OrganizationState>((set) => ({
  ...initialState,

  setOrganizations: (organizations) => set({ organizations }),

  setCurrentOrganization: (org) => set({ currentOrganization: org }),

  addOrganization: (org) =>
    set((state) => ({
      organizations: [...state.organizations, org],
    })),

  updateOrganization: (id, updates) =>
    set((state) => ({
      organizations: state.organizations.map((org) =>
        org.id === id ? { ...org, ...updates } : org
      ),
      currentOrganization:
        state.currentOrganization?.id === id
          ? { ...state.currentOrganization, ...updates }
          : state.currentOrganization,
    })),

  removeOrganization: (id) =>
    set((state) => ({
      organizations: state.organizations.filter((org) => org.id !== id),
      currentOrganization:
        state.currentOrganization?.id === id ? null : state.currentOrganization,
    })),

  setMembers: (members) => set({ members }),

  addMember: (member) =>
    set((state) => ({
      members: [...state.members, member],
    })),

  updateMember: (userId, updates) =>
    set((state) => ({
      members: state.members.map((m) =>
        m.user_id === userId ? { ...m, ...updates } : m
      ),
    })),

  removeMember: (userId) =>
    set((state) => ({
      members: state.members.filter((m) => m.user_id !== userId),
    })),

  setLoading: (isLoading) => set({ isLoading }),

  setError: (error) => set({ error }),

  reset: () => set(initialState),
}));
