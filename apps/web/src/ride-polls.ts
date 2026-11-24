export interface RidePollOption {
  id: string
  starts_at: string
  votes: number
}

export interface RidePoll {
  id: string
  city: string
  description: string
  timezone: string
  options: RidePollOption[]
  selected_option_id?: string
  my_option_ids: string[]
  can_manage: boolean
}

export interface RidePollResponse {
  poll: RidePoll
}

export const ridePollsPath = '/ride-polls'
export const newRidePollPath = ridePollsPath + '/new'

export function ridePollPath(id: string) {
  return ridePollsPath + '/' + encodeURIComponent(id)
}
