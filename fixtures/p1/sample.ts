// P1 Test Fixture: TypeScript Sample Code
// Purpose: Test TypeScript parsing accuracy for gdc sync --direction code
// Requirements: R3 (AC-R3-2) - Exported interface/class signatures extraction

/**
 * 사용자 인증 토큰 페이로드
 */
export interface TokenPayload {
  userId: string;
  email: string;
  roles: string[];
  exp: number;
}

/**
 * 인증 서비스 인터페이스
 */
export interface IAuthService {
  /**
   * 사용자 로그인
   * @param email 이메일
   * @param password 비밀번호
   * @returns JWT 토큰
   * @throws AuthError 인증 실패 시
   */
  login(email: string, password: string): Promise<string>;

  /**
   * 로그아웃
   * @param token 현재 세션 토큰
   */
  logout(token: string): Promise<void>;

  /**
   * 토큰 검증
   * @param token 검증할 토큰
   * @returns 페이로드 또는 null
   */
  validateToken(token: string): Promise<TokenPayload | null>;
}

/**
 * HTTP 클라이언트 설정
 */
export interface HttpClientConfig {
  baseURL: string;
  timeout: number;
  headers?: Record<string, string>;
  retryCount?: number;
}

/**
 * API 클라이언트 클래스
 */
export class ApiClient {
  private config: HttpClientConfig;
  private authToken: string | null = null;

  /**
   * ApiClient 생성자
   * @param config HTTP 설정
   */
  constructor(config: HttpClientConfig) {
    this.config = config;
  }

  /**
   * 인증 토큰 설정
   * @param token JWT 토큰
   */
  setAuthToken(token: string): void {
    this.authToken = token;
  }

  /**
   * GET 요청
   * @param path API 경로
   * @param params 쿼리 파라미터
   * @returns 응답 데이터
   */
  async get<T>(path: string, params?: Record<string, string>): Promise<T> {
    const url = this.buildURL(path, params);
    return this.request<T>('GET', url);
  }

  /**
   * POST 요청
   * @param path API 경로
   * @param body 요청 본문
   * @returns 응답 데이터
   */
  async post<T>(path: string, body: unknown): Promise<T> {
    const url = this.buildURL(path);
    return this.request<T>('POST', url, body);
  }

  /**
   * PUT 요청
   * @param path API 경로
   * @param body 요청 본문
   * @returns 응답 데이터
   */
  async put<T>(path: string, body: unknown): Promise<T> {
    const url = this.buildURL(path);
    return this.request<T>('PUT', url, body);
  }

  /**
   * DELETE 요청
   * @param path API 경로
   * @returns 성공 여부
   */
  async delete(path: string): Promise<boolean> {
    const url = this.buildURL(path);
    await this.request<unknown>('DELETE', url);
    return true;
  }

  /**
   * URL 구성
   * @param path API 경로
   * @param params 쿼리 파라미터
   * @returns 완성된 URL
   */
  private buildURL(path: string, params?: Record<string, string>): string {
    let url = `${this.config.baseURL}${path}`;
    if (params && Object.keys(params).length > 0) {
      const query = new URLSearchParams(params).toString();
      url += `?${query}`;
    }
    return url;
  }

  /**
   * HTTP 요청 실행
   * @param method HTTP 메서드
   * @param url 요청 URL
   * @param body 요청 본문 (optional)
   * @returns 응답 데이터
   */
  private async request<T>(
    method: string,
    url: string,
    body?: unknown
  ): Promise<T> {
    const headers: Record<string, string> = {
      'Content-Type': 'application/json',
      ...this.config.headers,
    };

    if (this.authToken) {
      headers['Authorization'] = `Bearer ${this.authToken}`;
    }

    const response = await fetch(url, {
      method,
      headers,
      body: body ? JSON.stringify(body) : undefined,
    });

    if (!response.ok) {
      throw new Error(`HTTP ${response.status}: ${response.statusText}`);
    }

    return response.json() as Promise<T>;
  }
}

/**
 * 사용자 저장소 클래스
 */
export class UserRepository {
  private apiClient: ApiClient;

  constructor(apiClient: ApiClient) {
    this.apiClient = apiClient;
  }

  /**
   * 사용자 조회
   * @param userId 사용자 ID
   * @returns 사용자 정보
   */
  async findById(userId: string): Promise<User | null> {
    try {
      return await this.apiClient.get<User>(`/users/${userId}`);
    } catch {
      return null;
    }
  }

  /**
   * 사용자 생성
   * @param user 생성할 사용자 데이터
   * @returns 생성된 사용자
   */
  async create(user: CreateUserRequest): Promise<User> {
    return this.apiClient.post<User>('/users', user);
  }
}

/**
 * 사용자 엔티티
 */
export interface User {
  id: string;
  email: string;
  name: string;
  createdAt: Date;
  updatedAt: Date;
}

/**
 * 사용자 생성 요청
 */
export interface CreateUserRequest {
  email: string;
  name: string;
  password: string;
}

/**
 * 인증 에러
 */
export class AuthError extends Error {
  constructor(message: string) {
    super(message);
    this.name = 'AuthError';
  }
}

// 남아있지만 export되지 않는 타입 (private)
interface InternalCache {
  get<T>(key: string): T | undefined;
  set<T>(key: string, value: T): void;
  clear(): void;
}
