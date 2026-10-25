export interface GraphNode {
  id: string
  kind: string
  label: string
  parent_id?: string
  status: string
  simulated: boolean
  technical_key: string
  properties: Record<string, unknown>
  last_seen_at: string
}

export interface GraphEdge {
  id: string
  source: string
  target: string
  kind: string
  evidence: string
  confidence: number
  status: string
  properties: Record<string, unknown>
  last_seen_at: string
}

export interface Graph {
  view: string
  available: boolean
  generated_at: string
  data_origin: 'live' | 'mixed' | 'fixture'
  notice?: string
  nodes: GraphNode[]
  edges: GraphEdge[]
}

export interface Activity {
  id: string
  kind: string
  message: string
  entity_id?: string
  simulated: boolean
  observed_at: string
}

export interface AgentInfo {
  id: string
  hostname: string
  version: string
  last_seen_at?: string
  status: string
}

export interface Summary {
  agents: AgentInfo[]
  entity_count: number
  data_origin: 'live' | 'mixed' | 'fixture'
}

export interface Position {
  x: number
  y: number
}
