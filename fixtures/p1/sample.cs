// P1 Test Fixture: C# Sample Code
// Purpose: Test C# parsing accuracy for gdc sync --direction code
// Requirements: R3 (AC-R3-1) - Public method signatures and dependencies extraction

namespace Gdc.TestFixtures
{
    /// <summary>
    /// 사용자 인증을 담당하는 서비스 인터페이스
    /// </summary>
    public interface IAuthService
    {
        /// <summary>
        /// 사용자 로그인
        /// </summary>
        /// <param name="username">사용자 이름</param>
        /// <param name="password">비밀번호</param>
        /// <returns>인증 토큰</returns>
        string Login(string username, string password);

        /// <summary>
        /// 사용자 로그아웃
        /// </summary>
        /// <param name="token">현재 세션 토큰</param>
        void Logout(string token);

        /// <summary>
        /// 토큰 유효성 검증
        /// </summary>
        /// <param name="token">검증할 토큰</param>
        /// <returns>유효 여부</returns>
        bool ValidateToken(string token);
    }

    /// <summary>
    /// 주문 처리 서비스
    /// </summary>
    public class OrderService
    {
        private readonly IAuthService _authService;
        private readonly IDatabase _database;
        private readonly ILogger<OrderService> _logger;

        /// <summary>
        /// OrderService 생성자
        /// </summary>
        /// <param name="authService">인증 서비스</param>
        /// <param name="database">데이터베이스 접근</param>
        /// <param name="logger">로거</param>
        public OrderService(
            IAuthService authService,
            IDatabase database,
            ILogger<OrderService> logger)
        {
            _authService = authService;
            _database = database;
            _logger = logger;
        }

        /// <summary>
        /// 새 주문 생성
        /// </summary>
        /// <param name="userId">사용자 ID</param>
        /// <param name="items">주문 아이템 목록</param>
        /// <param name="token">인증 토큰</param>
        /// <returns>생성된 주문 ID</returns>
        /// <exception cref="UnauthorizedAccessException">인증 실패</exception>
        /// <exception cref="ArgumentException">유효하지 않은 주문</exception>
        public string CreateOrder(string userId, List<OrderItem> items, string token)
        {
            if (!_authService.ValidateToken(token))
            {
                throw new UnauthorizedAccessException("Invalid token");
            }

            _logger.LogInformation($"Creating order for user {userId}");
            
            var order = new Order
            {
                UserId = userId,
                Items = items,
                CreatedAt = DateTime.UtcNow
            };

            return _database.Save(order);
        }

        /// <summary>
        /// 주문 상태 조회
        /// </summary>
        /// <param name="orderId">주문 ID</param>
        /// <param name="token">인증 토큰</param>
        /// <returns>주문 정보</returns>
        public Order GetOrder(string orderId, string token)
        {
            if (!_authService.ValidateToken(token))
            {
                throw new UnauthorizedAccessException("Invalid token");
            }

            return _database.Get<Order>(orderId);
        }

        /// <summary>
        /// 주문 취소
        /// </summary>
        /// <param name="orderId">취소할 주문 ID</param>
        /// <param name="token">인증 토큰</param>
        /// <returns>취소 성공 여부</returns>
        public bool CancelOrder(string orderId, string token)
        {
            if (!_authService.ValidateToken(token))
            {
                return false;
            }

            _logger.LogInformation($"Cancelling order {orderId}");
            return _database.Delete<Order>(orderId);
        }
    }

    /// <summary>
    /// 주문 아이템
    /// </summary>
    public class OrderItem
    {
        public string ProductId { get; set; }
        public int Quantity { get; set; }
        public decimal UnitPrice { get; set; }
    }

    /// <summary>
    /// 주문 엔티티
    /// </summary>
    public class Order
    {
        public string Id { get; set; }
        public string UserId { get; set; }
        public List<OrderItem> Items { get; set; }
        public DateTime CreatedAt { get; set; }
        public OrderStatus Status { get; set; }
    }

    /// <summary>
    /// 주문 상태
    /// </summary>
    public enum OrderStatus
    {
        Pending,
        Processing,
        Shipped,
        Delivered,
        Cancelled
    }

    // 지원 인터페이스 (의존성으로만 사용)
    public interface IDatabase
    {
        string Save<T>(T entity);
        T Get<T>(string id);
        bool Delete<T>(string id);
    }

    public interface ILogger<T>
    {
        void LogInformation(string message);
        void LogError(string message);
    }
}
