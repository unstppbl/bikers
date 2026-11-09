export type Locale = 'en' | 'ru' | 'kk'

export interface Profile {
  display_name: string
  city: string
  languages: string[]
  interests: string
  locale: Locale
}

export interface User {
  id: string
  profile: Profile
  onboarding_complete: boolean
}

export interface Session {
  user: User | null
  csrf_token?: string
}

export interface Challenge {
  challenge_id: string
  expires_at: string
  resend_after: string
}

export interface MapPoint { label: string; lat: number; lon: number }
export interface MapGeometry { type: 'LineString'; coordinates: [number, number][] }
export interface MapPlan { points: MapPoint[]; geometry?: MapGeometry; distance_meters: number; duration_seconds: number }
export interface MapPlace extends MapPoint { id: string; kind: string }
export interface MapConfig { enabled: boolean; archive_url: string }

export interface Ride {
  id: string
  city: string
  date: string
  description: string
  status: string
  participation_state?: string
  can_manage?: boolean
  title?: string
  starts_at?: string
  timezone?: string
  meeting_point?: string
  destination?: string
  route_url?: string
  expectations?: string
  capacity?: number
  plan_version?: number
  organizer_id?: string
  map_plan?: MapPlan
}

export interface RidePage { rides: Ride[]; next_cursor: string }

export interface RideInput {
  title: string
  city: string
  description: string
  starts_at: string
  timezone: string
  meeting_point: string
  destination: string
  route_url: string
  expectations: string
  capacity: number
  plan_version?: number
  map_points?: MapPoint[]
}

export interface Participant {
  user_id: string
  display_name: string
  state: string
}

export interface Message {
  id: string
  sequence: number
  sender_id: string
  sender_name: string
  body: string
  created_at: string
  client_message_id: string
}

export interface MessagesResponse {
  messages: Message[]
  next_sequence: number
}

export interface BikeModelImage {
  url: string; year_from: number; year_to: number; width: number; height: number
  source_url: string; author: string; license: string; license_url: string
}
export interface BikeModel { id: string; make: string; model: string; aliases: string[]; images: BikeModelImage[] }
export interface PublicBike { id: string; make: string; model: string; model_id: string; year: number }
export interface Bike extends PublicBike {
  owner_id: string; visibility: 'private' | 'public'; version: number; created_at: string; updated_at: string; can_manage: true
}
export interface BikePhoto { id: string; url: string; width: number; height: number; created_at: string }
export interface BikePhotoGallery { photos: BikePhoto[]; cover_photo_id: string; version?: number }
export interface MaintenanceEntry {
  id: string; bike_id: string; service_date: string; mileage_km: number; work_performed: string; version: number; created_at: string; updated_at: string
}
export interface ClubSummary {
  id: string; name: string; city: string; description: string; status: 'active' | 'closed'; membership_state?: string; can_manage?: boolean
}
export interface Club extends ClubSummary { owner_id: string; version: number; member_count: number }
export interface ClubMember { user_id: string; display_name: string; state: string; role: 'owner' | 'member' }
export interface ClubImagePhoto { id: string; url: string; width: number; height: number; created_at: string }
export interface ClubImageMetadata { image: ClubImagePhoto | null; version?: number }
export interface AvatarMetadata { image: BikePhoto | null; version?: number }
export interface RidePhoto extends BikePhoto { uploader_id: string; can_delete: boolean; hidden?: boolean }
export interface RideVideo { id: string; url: string; title: string; uploader_id: string; created_at: string; can_delete: boolean; hidden?: boolean }
export interface RideAlbum { photos: RidePhoto[]; videos: RideVideo[]; version: number }
export interface Announcement { id: string; sequence: number; owner_id: string; owner_name: string; body: string; created_at: string; client_announcement_id: string }
export interface InboxNotification { id: string; kind: string; ride_id?: string; club_id?: string; created_at: string; read_at: string | null }
export interface NotificationPage { notifications: InboxNotification[]; next_cursor: string; unread_count: number }
export interface BlockedRider { user_id: string; display_name: string; created_at: string }
export type ReportTarget = 'user' | 'ride' | 'club' | 'message' | 'bike' | 'bike_photo' | 'club_image' | 'avatar' | 'ride_photo' | 'ride_video' | 'announcement'

export interface PushCategories { ride_updates: boolean; participation: boolean; club_membership: boolean; ride_reminders: boolean }
export interface RideReminders { day_before: boolean; hour_before: boolean }

export type PlaceCategory = 'fuel' | 'workshop' | 'tires' | 'motorcycle_shop' | 'parking' | 'meeting_spot'
export interface PlaceInput {
  name: string; category: PlaceCategory; lat: number; lon: number; city: string; address: string; description: string; website: string; phone: string; opening_hours: string
}
export interface Place extends PlaceInput {
  id: string; source: 'osm' | 'community'; source_ref?: string; source_url: string; source_date?: string; updated_at: string; version: number; status: 'published'
}
export interface PlaceBounds { west: number; south: number; east: number; north: number }
export type PlaceSubmissionKind = 'new' | 'correction' | 'closed'
export interface PlaceSubmissionInput { kind: PlaceSubmissionKind; place_id?: string; base_version?: number; place?: PlaceInput; reason: string }
export interface PlaceSubmission extends PlaceSubmissionInput { id: string; status: 'pending' | 'approved' | 'rejected'; version: number; created_at: string; updated_at: string; review_reason?: string; submitter_id?: string; reviewer_id?: string }
