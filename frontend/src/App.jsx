import { useMemo, useState } from 'react';

const EMAIL_REGEX = /^[^\s@]+@[^\s@]+\.[^\s@]+$/;

export default function App() {
  const [mode, setMode] = useState('login');
  const [form, setForm] = useState({
    name: '',
    email: '',
    phone: '',
    password: '',
    confirmPassword: '',
  });
  const [touched, setTouched] = useState({});
  const [submitted, setSubmitted] = useState(false);
  const [loading, setLoading] = useState(false);
  const [apiError, setApiError] = useState('');

  const isRegister = mode === 'register';

  const errors = useMemo(() => {
    const nextErrors = {};

    if (isRegister && form.name.trim().length < 2) {
      nextErrors.name = 'Введите имя (минимум 2 символа).';
    }

    if (!EMAIL_REGEX.test(form.email)) {
      nextErrors.email = 'Введите корректный email.';
    }

    if (isRegister && !/^\+?[0-9\s()-]{10,}$/.test(form.phone)) {
      nextErrors.phone = 'Введите корректный номер телефона.';
    }

    if (form.password.length < 6) {
      nextErrors.password = 'Пароль должен быть не короче 6 символов.';
    }

    if (isRegister && form.confirmPassword !== form.password) {
      nextErrors.confirmPassword = 'Пароли не совпадают.';
    }

    return nextErrors;
  }, [form, isRegister]);

  const hasErrors = Object.keys(errors).length > 0;

  const handleChange = (event) => {
    const { name, value } = event.target;
    setForm((prev) => ({ ...prev, [name]: value }));
  };

  const handleBlur = (event) => {
    const { name } = event.target;
    setTouched((prev) => ({ ...prev, [name]: true }));
  };

  const handleModeChange = (nextMode) => {
    setMode(nextMode);
    setSubmitted(false);
    setApiError('');
    setTouched({});
  };

  const handleSubmit = async (event) => {
    event.preventDefault();
    setApiError('');
    setSubmitted(false);

    setTouched({
      name: true,
      email: true,
      phone: true,
      password: true,
      confirmPassword: true,
    });

    if (hasErrors) {
      return;
    }

    setLoading(true);

    try {
      if (isRegister) {
        const response = await fetch('/api/auth/register', {
          method: 'POST',
          headers: {
            'Content-Type': 'application/json',
          },
          body: JSON.stringify({
            name: form.name,
            phone: form.phone,
            email: form.email,
            password: form.password,
            confirmPassword: form.confirmPassword,
          }),
        });

        const data = await response.json().catch(() => ({}));
        if (!response.ok) {
          throw new Error(data.error || 'Не удалось зарегистрироваться.');
        }

        setSubmitted(true);
      } else {
        const response = await fetch('/api/auth/login', {
          method: 'POST',
          headers: {
            'Content-Type': 'application/json',
          },
          body: JSON.stringify({
            login: form.email,
            password: form.password,
          }),
        });

        const data = await response.json().catch(() => ({}));
        if (!response.ok) {
          throw new Error(data.error || 'Не удалось выполнить вход.');
        }

        if (data.token) {
          localStorage.setItem('auth_token', data.token);
        }

        setSubmitted(true);
      }
    } catch (error) {
      setApiError(error.message || 'Произошла ошибка запроса.');
    } finally {
      setLoading(false);
    }
  };

  return (
    <main className="page">
      <section className="hero-card">
        <div className="brand-block">
          <p className="brand-eyebrow">Delivery Service</p>
          <h1>Быстрый вход в систему доставки</h1>
          <p>
            Управляйте заявками, отслеживайте курьеров и контролируйте заказы в
            одном месте.
          </p>
          <div className="chip-row">
            <span>Безопасная авторизация</span>
            <span>24/7 доступ</span>
            <span>Онлайн-трекинг</span>
          </div>
        </div>

        <div className="form-shell">
          <div className="mode-switch" role="tablist" aria-label="Выбор режима">
            <span
              className={`mode-indicator ${isRegister ? 'register' : 'login'}`}
              aria-hidden="true"
            />
            <button
              type="button"
              className={mode === 'login' ? 'active' : ''}
              onClick={() => handleModeChange('login')}
            >
              Вход
            </button>
            <button
              type="button"
              className={mode === 'register' ? 'active' : ''}
              onClick={() => handleModeChange('register')}
            >
              Регистрация
            </button>
          </div>

          <form
            className={`auth-form ${isRegister ? 'register' : 'login'}`}
            onSubmit={handleSubmit}
            noValidate
          >
            <div className={`field collapse-field ${isRegister ? 'open' : ''}`}>
              <label htmlFor="name">Имя</label>
              <input
                id="name"
                name="name"
                type="text"
                placeholder="Иван"
                value={form.name}
                onChange={handleChange}
                onBlur={handleBlur}
                disabled={!isRegister}
              />
              {isRegister && touched.name && errors.name && (
                <p className="error">{errors.name}</p>
              )}
            </div>

            <div className="field">
              <label htmlFor="email">Email</label>
              <input
                id="email"
                name="email"
                type="email"
                placeholder="you@example.com"
                value={form.email}
                onChange={handleChange}
                onBlur={handleBlur}
              />
              {touched.email && errors.email && <p className="error">{errors.email}</p>}
            </div>

            <div className={`field collapse-field ${isRegister ? 'open' : ''}`}>
              <label htmlFor="phone">Телефон</label>
              <input
                id="phone"
                name="phone"
                type="tel"
                placeholder="+7 (999) 123-45-67"
                value={form.phone}
                onChange={handleChange}
                onBlur={handleBlur}
                disabled={!isRegister}
              />
              {isRegister && touched.phone && errors.phone && (
                <p className="error">{errors.phone}</p>
              )}
            </div>

            <div className="field">
              <label htmlFor="password">Пароль</label>
              <input
                id="password"
                name="password"
                type="password"
                placeholder="••••••••"
                value={form.password}
                onChange={handleChange}
                onBlur={handleBlur}
              />
              {touched.password && errors.password && (
                <p className="error">{errors.password}</p>
              )}
            </div>

            <div className={`field collapse-field ${isRegister ? 'open' : ''}`}>
              <label htmlFor="confirmPassword">Повторите пароль</label>
              <input
                id="confirmPassword"
                name="confirmPassword"
                type="password"
                placeholder="••••••••"
                value={form.confirmPassword}
                onChange={handleChange}
                onBlur={handleBlur}
                disabled={!isRegister}
              />
              {isRegister && touched.confirmPassword && errors.confirmPassword && (
                <p className="error">{errors.confirmPassword}</p>
              )}
            </div>

            <button type="submit" className="submit-btn" disabled={loading}>
              {loading
                ? 'Отправка...'
                : isRegister
                  ? 'Создать аккаунт'
                  : 'Войти'}
            </button>

            {apiError && <p className="api-error">{apiError}</p>}

            {submitted && (
              <p className="success">
                {isRegister
                  ? 'Регистрация успешна! Теперь можно войти.'
                  : 'Вход успешен!'}
              </p>
            )}
          </form>
        </div>
      </section>
    </main>
  );
}
