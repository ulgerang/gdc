use std::sync::Arc;

pub trait UserRepository {
    fn find_by_id(&self, id: &str) -> Result<User, RepoError>;
    fn save(&self, user: User) -> Result<(), RepoError>;
}

pub struct UserService {
    repo: Arc<dyn UserRepository>,
    logger: Logger,
}

impl UserService {
    pub fn new(repo: Arc<dyn UserRepository>, logger: Logger) -> Self {
        Self { repo, logger }
    }

    pub async fn get_user(&self, id: &str) -> Result<User, ServiceError> {
        todo!()
    }
}
