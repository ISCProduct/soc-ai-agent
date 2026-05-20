import { authService } from '@/lib/auth'

const API_BASE = '/api'

function unwrapArray<T>(raw: unknown): T[] {
    if (Array.isArray(raw)) return raw as T[]
    const obj = raw as Record<string, unknown>
    if (obj && Array.isArray(obj.data)) return obj.data as T[]
    return []
}

export interface ChatRequest {
    user_id: number
    session_id: string
    message: string
    industry_id: number
    job_category_id: number
    chat_history?: Array<{
        role: "user" | "assistant"
        content: string
    }>
}

export interface PhaseProgress {
    phase_id: number
    phase_name: string
    display_name: string
    questions_asked: number
    valid_answers: number
    completion_score: number
    is_completed: boolean
    min_questions: number
    max_questions: number
}

export interface ChatResponse {
    response: string
    question_weight_id?: number
    is_complete: boolean
    is_terminated?: boolean
    invalid_answer_count?: number
    total_questions: number
    answered_questions: number
    evaluated_categories?: number
    total_categories?: number
    current_phase?: PhaseProgress
    all_phases?: PhaseProgress[]
    current_scores?: Array<{
        id: number
        user_id: number
        session_id: string
        weight_category?: string
        category: string
        score: number
        reason?: string
        created_at: string
        updated_at: string
    }>
}

export interface ChatScore {
    id: number
    user_id: number
    session_id: string
    weight_category?: string
    category: string
    score: number
    reason?: string
    created_at: string
    updated_at: string
}

export interface ChatHistory {
    id: number
    session_id: string
    user_id: number
    role: "user" | "assistant"
    content: string
    question_weight_id?: number
    created_at: string
}

export async function sendChatMessage(request: ChatRequest): Promise<ChatResponse> {
    try {
        const response = await fetch(`${API_BASE}/chat`, {
            method: "POST",
            headers: {
                "Content-Type": "application/json",
                ...authService.getUserFetchHeaders(),
            },
            body: JSON.stringify(request),
        })

        if (!response.ok) {
            const errorText = await response.text().catch(() => response.statusText)
            console.error('[API] Chat error:', response.status, errorText)
            throw new Error(`Chat API error: ${errorText || response.statusText}`)
        }

        return response.json()
    } catch (error) {
        console.error('[API] Failed to send chat message:', error)
        throw error
    }
}

export async function getChatHistory(sessionId: string): Promise<ChatHistory[]> {
    try {
        const response = await fetch(`${API_BASE}/chat/history?session_id=${sessionId}`, {
            headers: authService.getUserFetchHeaders(),
        })

        if (!response.ok) {
            console.warn(`History API error: ${response.status}`)
            return []
        }

        const raw = await response.json()
        return unwrapArray<ChatHistory>(raw)
    } catch (error) {
        console.warn('Failed to fetch chat history:', error)
        return []
    }
}

export async function getUserScores(sessionId: string): Promise<ChatScore[]> {
    const response = await fetch(`${API_BASE}/chat/scores?session_id=${sessionId}`, {
        headers: authService.getUserFetchHeaders(),
    })

    if (!response.ok) {
        throw new Error(`Scores API error: ${response.statusText}`)
    }

    const raw = await response.json()
    return unwrapArray<ChatScore>(raw)
}

export async function getRecommendations(sessionId: string, limit = 5) {
    const response = await fetch(
        `${API_BASE}/chat/recommendations?session_id=${sessionId}&limit=${limit}`,
        { headers: authService.getUserFetchHeaders() },
    )

    if (!response.ok) {
        throw new Error(`Recommendations API error: ${response.statusText}`)
    }

    return response.json()
}

export async function sendMessage(message: string): Promise<{ message: string }> {
    const sessionId = `session_${Date.now()}_${Math.random().toString(36).substring(7)}`
    
    try {
        const response = await sendChatMessage({
            user_id: 1,
            session_id: sessionId,
            message,
            industry_id: 1,
            job_category_id: 0,
        })
        
        return {
            message: response.response || 'メッセージを受信しました。',
        }
    } catch (error) {
        throw error
    }
}

export async function sendAnalysisReport(userId: number, sessionId: string): Promise<{ message: string }> {
    const response = await fetch(`${API_BASE}/chat/send-report`, {
        method: 'POST',
        headers: {
            'Content-Type': 'application/json',
            ...authService.getUserFetchHeaders(),
        },
        body: JSON.stringify({ session_id: sessionId }),
    })

    if (!response.ok) {
        const data = await response.json().catch(() => ({}))
        throw new Error(data.error || `Send report error: ${response.statusText}`)
    }

    return response.json()
}

export async function getCompanyDetail(companyId: number) {
    const response = await fetch(`${API_BASE}/companies/${companyId}`)
    
    if (!response.ok) {
        throw new Error(`Company API error: ${response.statusText}`)
    }
    
    return response.json()
}
