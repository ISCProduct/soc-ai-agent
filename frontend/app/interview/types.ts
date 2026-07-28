/**
 * Shared types for the interview feature.
 */

export type Utterance = {
  role: 'user' | 'ai'
  text: string
}

export type InterviewCompany = {
  id: number
  name: string
  name_reading?: string
  description?: string
  main_business?: string
  industry?: string
  location?: string
  employee_count?: number
  culture?: string
  work_style?: string
  welfare_details?: string
}

export type Position = {
  id: string
  title: string
  department: string
  icon: string
  questions: number
  category: 'general' | 'sier'
}

export type InterviewStatus = 'selection' | 'lobby' | 'connecting' | 'connected' | 'error' | 'finished'
