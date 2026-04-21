import { useEffect, useMemo, useState } from 'react';
import { CircleMarker, MapContainer, Polyline, TileLayer, Tooltip, useMapEvents } from 'react-leaflet';

const STATUS_FLOW = ['Создан', 'Курьер в пути', 'Забран у отправителя', 'Доставлен'];
const STORAGE_TOKEN_KEY = 'delivery_token';
const STORAGE_PROFILE_KEY = 'delivery_profile';

// Pricing constants (must match backend defaults)
const PRICING = {
  BASE_RATE: 150,
  PER_KM_RATE: 20,
  PER_KG_RATE: 50,
};

const computePrice = (distanceKm, weightKg) => {
  const distanceBilled = Math.ceil(distanceKm);
  return Math.round((PRICING.BASE_RATE + distanceBilled * PRICING.PER_KM_RATE + weightKg * PRICING.PER_KG_RATE) * 100) / 100;
};

const INITIAL_ORDERS = [
  {
    id: 'DLV-24018',
    from: 'ул. Тверская, 14',
    to: 'Ленинский проспект, 30',
    fromCoords: [55.7608, 37.6133],
    toCoords: [55.706, 37.5895],
    status: 'Курьер в пути',
    eta: '17 мин',
  },
  {
    id: 'DLV-24012',
    from: 'пр. Мира, 102',
    to: 'Садовая-Самотечная, 7',
    fromCoords: [55.7943, 37.6348],
    toCoords: [55.7744, 37.615],
    status: 'Создан',
    eta: '26 мин',
  },
];

const DEFAULT_FROM_COORDS = [55.7558, 37.6176];
const DEFAULT_TO_COORDS = [55.706, 37.5895];
const parseCoords = (value) => {
  const normalized = value.trim().replace(';', ',');
  const parts = normalized.split(',').map((chunk) => chunk.trim());
  if (parts.length !== 2) {
    return null;
  }

  const lat = Number(parts[0]);
  const lon = Number(parts[1]);
  if (!Number.isFinite(lat) || !Number.isFinite(lon)) {
    return null;
  }
  if (lat < -90 || lat > 90 || lon < -180 || lon > 180) {
    return null;
  }

  return [lat, lon];
};

const routeToEta = (seconds) => `${Math.max(1, Math.round(seconds / 60))} мин`;

function MapClickHandler({ onPick }) {
  useMapEvents({
    click(event) {
      onPick([Number(event.latlng.lat.toFixed(6)), Number(event.latlng.lng.toFixed(6))]);
    },
  });

  return null;
}

export default function App() {
  const [session, setSession] = useState(() => {
    const token = localStorage.getItem(STORAGE_TOKEN_KEY);
    const profileRaw = localStorage.getItem(STORAGE_PROFILE_KEY);
    let profile = { name: '', phone: '', email: '' };

    if (profileRaw) {
      try {
        profile = { ...profile, ...JSON.parse(profileRaw) };
      } catch {
        profile = { name: '', phone: '', email: '' };
      }
    }

    return {
      token: token || '',
      profile,
    };
  });
  const [currentView, setCurrentView] = useState(() => (localStorage.getItem(STORAGE_TOKEN_KEY) ? 'order' : 'register'));
  const [authMode, setAuthMode] = useState('register');
  const [authLoading, setAuthLoading] = useState(false);
  const [authError, setAuthError] = useState('');
  const [authNotice, setAuthNotice] = useState('');
  const [authForm, setAuthForm] = useState({
    name: '',
    phone: '',
    email: '',
    password: '',
    confirmPassword: '',
    login: '',
  });
  const [profileForm, setProfileForm] = useState(() => ({
    name: session.profile.name || '',
    phone: session.profile.phone || '',
    email: session.profile.email || '',
  }));
  const [profileNotice, setProfileNotice] = useState('');

  const [form, setForm] = useState({
    from: '',
    to: '',
    fromCoords: '',
    toCoords: '',
    phone: '',
    weight: '',
    pickupTime: 'Как можно скорее',
    payment: 'card',
    comment: '',
  });
  const [computedPrice, setComputedPrice] = useState(null);
  const [orders, setOrders] = useState(INITIAL_ORDERS);
  const [activeOrderId, setActiveOrderId] = useState(INITIAL_ORDERS[0].id);
  const [formError, setFormError] = useState('');
  const [banner, setBanner] = useState('');
  const [routeInfo, setRouteInfo] = useState({
    loading: false,
    distanceKm: null,
    durationMin: null,
    geometry: [],
    error: '',
  });
  const [mapEditTarget, setMapEditTarget] = useState('from');
  const [mapExpanded, setMapExpanded] = useState(false);
  const [geoStatus, setGeoStatus] = useState({ loading: false, error: '' });
  const [addressSuggestions, setAddressSuggestions] = useState({ from: [], to: [] });

  const activeOrder = useMemo(
    () => orders.find((order) => order.id === activeOrderId) ?? orders[0],
    [orders, activeOrderId],
  );

  const activeStep = STATUS_FLOW.indexOf(activeOrder?.status || STATUS_FLOW[0]);
  const completedOrders = orders.filter((order) => order.status === 'Доставлен').length;
  const inProgressOrders = orders.filter((order) => order.status !== 'Доставлен').length;
  const recentOrders = orders.slice(0, 3);
  const mapCenter = useMemo(() => {
    const from = activeOrder?.fromCoords || DEFAULT_FROM_COORDS;
    const to = activeOrder?.toCoords || DEFAULT_TO_COORDS;
    return [Number(((from[0] + to[0]) / 2).toFixed(6)), Number(((from[1] + to[1]) / 2).toFixed(6))];
  }, [activeOrder]);

  const updateSession = (nextSession) => {
    setSession(nextSession);

    if (nextSession.token) {
      localStorage.setItem(STORAGE_TOKEN_KEY, nextSession.token);
    } else {
      localStorage.removeItem(STORAGE_TOKEN_KEY);
    }

    localStorage.setItem(STORAGE_PROFILE_KEY, JSON.stringify(nextSession.profile || { name: '', phone: '', email: '' }));
  };

  const handleAuthChange = (event) => {
    const { name, value } = event.target;
    setAuthForm((prev) => ({ ...prev, [name]: value }));
  };

  const handleProfileChange = (event) => {
    const { name, value } = event.target;
    setProfileForm((prev) => ({ ...prev, [name]: value }));
  };

  const loginWithCredentials = async (login, password) => {
    const response = await fetch('/api/auth/login', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ login, password }),
    });

    if (!response.ok) {
      const body = await response.text();
      throw new Error(body || `HTTP ${response.status}`);
    }

    return response.json();
  };

  const handleAuthSubmit = async (event) => {
    event.preventDefault();
    setAuthError('');
    setAuthNotice('');

    if (authMode === 'register') {
      if (!authForm.name.trim() || !authForm.phone.trim() || !authForm.email.trim()) {
        setAuthError('Заполни имя, телефон и email.');
        return;
      }
      if (authForm.password.length < 8) {
        setAuthError('Пароль должен быть не короче 8 символов.');
        return;
      }
      if (authForm.password !== authForm.confirmPassword) {
        setAuthError('Пароли не совпадают.');
        return;
      }
    } else if (!authForm.login.trim() || !authForm.password.trim()) {
      setAuthError('Введи логин (email/телефон) и пароль.');
      return;
    }

    setAuthLoading(true);

    try {
      if (authMode === 'register') {
        const registerResponse = await fetch('/api/auth/register', {
          method: 'POST',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify({
            name: authForm.name.trim(),
            phone: authForm.phone.trim(),
            email: authForm.email.trim(),
            password: authForm.password,
            confirmPassword: authForm.confirmPassword,
          }),
        });

        if (!registerResponse.ok) {
          const body = await registerResponse.text();
          throw new Error(body || `HTTP ${registerResponse.status}`);
        }

        const loginData = await loginWithCredentials(authForm.email.trim(), authForm.password);
        const nextProfile = {
          name: authForm.name.trim(),
          phone: authForm.phone.trim(),
          email: authForm.email.trim(),
        };

        updateSession({ token: loginData.token || '', profile: nextProfile });
        setProfileForm(nextProfile);
        setCurrentView('order');
        setAuthNotice('Регистрация и вход выполнены.');
      } else {
        const loginData = await loginWithCredentials(authForm.login.trim(), authForm.password);
        const nextProfile = {
          name: session.profile.name || '',
          phone: session.profile.phone || '',
          email: authForm.login.includes('@') ? authForm.login.trim() : session.profile.email || '',
        };

        updateSession({ token: loginData.token || '', profile: nextProfile });
        setProfileForm(nextProfile);
        setCurrentView('order');
        setAuthNotice('Вход выполнен.');
      }
    } catch (_error) {
      setAuthError('Ошибка авторизации. Проверь данные и доступность auth-service.');
    } finally {
      setAuthLoading(false);
    }
  };

  const handleSaveProfile = (event) => {
    event.preventDefault();

    const nextProfile = {
      name: profileForm.name.trim(),
      phone: profileForm.phone.trim(),
      email: profileForm.email.trim(),
    };

    updateSession({ ...session, profile: nextProfile });
    setProfileNotice('Профиль сохранён.');
  };

  const handleLogout = () => {
    updateSession({ token: '', profile: session.profile });
    setCurrentView('register');
    setAuthMode('register');
  };

  useEffect(() => {
    if (!activeOrder) {
      return;
    }

    const fromCoords = activeOrder.fromCoords || DEFAULT_FROM_COORDS;
    const toCoords = activeOrder.toCoords || DEFAULT_TO_COORDS;
    const controller = new AbortController();
    const sleep = (ms) => new Promise((resolve) => setTimeout(resolve, ms));
    const maxAttempts = 60;
    const retryDelayMs = 5000;

    const loadRoute = async () => {
      setRouteInfo({ loading: true, distanceKm: null, durationMin: null, geometry: [], error: '' });

      for (let attempt = 1; attempt <= maxAttempts; attempt += 1) {
        try {
          const query = new URLSearchParams({
            fromLat: String(fromCoords[0]),
            fromLon: String(fromCoords[1]),
            toLat: String(toCoords[0]),
            toLon: String(toCoords[1]),
          });
          const response = await fetch(
            `/api/route?${query.toString()}`,
            { signal: controller.signal },
          );

          if (!response.ok) {
            const responseText = await response.text();
            const routeError = new Error(responseText || `HTTP ${response.status}`);
            routeError.status = response.status;
            throw routeError;
          }

          const data = await response.json();
          if (!Number.isFinite(data.distance) || !Number.isFinite(data.duration)) {
            throw new Error('Маршрут не найден');
          }

          const geometry = Array.isArray(data.geometry?.coordinates)
            ? data.geometry.coordinates
                .filter((pair) =>
                  Array.isArray(pair) &&
                  pair.length >= 2 &&
                  Number.isFinite(pair[0]) &&
                  Number.isFinite(pair[1]),
                )
                .map((pair) => [pair[1], pair[0]])
            : [];

          const distanceKm = Number((data.distance / 1000).toFixed(1));
          const durationMin = Math.max(1, Math.round(data.duration / 60));
          setRouteInfo({ loading: false, distanceKm, durationMin, geometry, error: '' });

          const eta = routeToEta(data.duration);
          setOrders((prev) =>
            prev.map((order) =>
              order.id === activeOrder.id && order.eta !== eta ? { ...order, eta } : order,
            ),
          );
          return;
        } catch (error) {
          if (error.name === 'AbortError') {
            return;
          }

          const status = Number(error.status) || 0;
          const retryable = status === 0 || status >= 500;

          if (attempt < maxAttempts && retryable) {
            await sleep(retryDelayMs);
            continue;
          }

          const reason = error.message || 'неизвестная ошибка';
          const fallbackMessage = status === 404
            ? 'Маршрут не найден для выбранных точек.'
            : `Маршрут недоступен: ${reason}. Проверь /api/route и контейнеры auth-service + osrm.`;

          setRouteInfo({
            loading: false,
            distanceKm: null,
            durationMin: null,
            geometry: [],
            error: fallbackMessage,
          });
        }
      }
    };

    loadRoute();

    return () => {
      controller.abort();
    };
  }, [activeOrder]);

  // Compute price preview when distance and weight change
  useEffect(() => {
    if (routeInfo.distanceKm === null) {
      setComputedPrice(null);
      return;
    }
    const weight = parseFloat(form.weight) || 0;
    setComputedPrice(computePrice(routeInfo.distanceKm, weight));
  }, [routeInfo.distanceKm, form.weight]);

  const handleMapClick = (nextCoords) => {
    if (!activeOrder) {
      return;
    }

    setOrders((prev) =>
      prev.map((order) =>
        order.id === activeOrder.id
          ? mapEditTarget === 'from'
            ? { ...order, fromCoords: nextCoords }
            : { ...order, toCoords: nextCoords }
          : order,
      ),
    );

    setBanner(
      mapEditTarget === 'from'
        ? `Точка A обновлена: ${nextCoords[0]}, ${nextCoords[1]}`
        : `Точка B обновлена: ${nextCoords[0]}, ${nextCoords[1]}`,
    );
  };

  const handleChange = (event) => {
    const { name, value } = event.target;
    setForm((prev) => ({ ...prev, [name]: value }));

    if (name === 'from' || name === 'to') {
      setAddressSuggestions((prev) => ({ ...prev, [name]: [] }));
    }
  };

  const fetchAddressSuggestions = async (target) => {
    const queryValue = target === 'from' ? form.from.trim() : form.to.trim();
    if (queryValue.length < 3) {
      setGeoStatus({ loading: false, error: 'Введи минимум 3 символа для поиска адреса.' });
      return;
    }

    setGeoStatus({ loading: true, error: '' });

    try {
      const params = new URLSearchParams({ query: queryValue });
      const response = await fetch(`/api/geocode/suggest?${params.toString()}`);
      if (!response.ok) {
        throw new Error(`HTTP ${response.status}`);
      }

      const data = await response.json();
      const suggestions = Array.isArray(data.suggestions) ? data.suggestions : [];
      setAddressSuggestions((prev) => ({ ...prev, [target]: suggestions }));
      setGeoStatus({ loading: false, error: suggestions.length ? '' : 'Варианты не найдены. Уточни адрес.' });
    } catch (_error) {
      setGeoStatus({ loading: false, error: 'Не удалось получить список адресов.' });
    }
  };

  const applySuggestion = (target, suggestion) => {
    const coordsText = `${suggestion.lat},${suggestion.lon}`;
    const coords = [suggestion.lat, suggestion.lon];
    setForm((prev) =>
      target === 'from'
        ? { ...prev, from: suggestion.display_name, fromCoords: coordsText }
        : { ...prev, to: suggestion.display_name, toCoords: coordsText },
    );

    if (activeOrder) {
      setOrders((prev) =>
        prev.map((order) =>
          order.id === activeOrder.id
            ? target === 'from'
              ? { ...order, from: suggestion.display_name, fromCoords: coords }
              : { ...order, to: suggestion.display_name, toCoords: coords }
            : order,
        ),
      );
    }

    setAddressSuggestions((prev) => ({ ...prev, [target]: [] }));
    setGeoStatus({ loading: false, error: '' });
    setBanner(target === 'from' ? `Точка A выбрана из списка: ${coordsText}` : `Точка B выбрана из списка: ${coordsText}`);
  };

  const geocodeAddress = async (address) => {
    const query = new URLSearchParams({ address });
    const response = await fetch(`/api/geocode?${query.toString()}`);
    if (!response.ok) {
      throw new Error(`HTTP ${response.status}`);
    }

    const data = await response.json();
    if (!Number.isFinite(data.lat) || !Number.isFinite(data.lon)) {
      throw new Error('Геокодер не вернул координаты');
    }

    return [data.lat, data.lon];
  };

  const geocodeFormPoint = async (target) => {
    const address = target === 'from' ? form.from.trim() : form.to.trim();
    if (address.length < 3) {
      setFormError(target === 'from' ? 'Введите адрес отправления для геокодирования.' : 'Введите адрес доставки для геокодирования.');
      return;
    }

    setGeoStatus({ loading: true, error: '' });
    setFormError('');

    try {
      const coords = await geocodeAddress(address);
      setForm((prev) =>
        target === 'from'
          ? { ...prev, fromCoords: `${coords[0]},${coords[1]}` }
          : { ...prev, toCoords: `${coords[0]},${coords[1]}` },
      );

      if (activeOrder) {
        setOrders((prev) =>
          prev.map((order) =>
            order.id === activeOrder.id
              ? target === 'from'
                ? { ...order, from: address, fromCoords: coords }
                : { ...order, to: address, toCoords: coords }
              : order,
          ),
        );
      }

      setBanner(target === 'from' ? `Точка A определена: ${coords[0]}, ${coords[1]}` : `Точка B определена: ${coords[0]}, ${coords[1]}`);
      setGeoStatus({ loading: false, error: '' });
    } catch (_error) {
      setGeoStatus({ loading: false, error: 'Не удалось найти адрес. Попробуй: улица + дом + Москва.' });
      setFormError('Адрес не найден. Пример: Тверская 14, Москва');
    }
  };

  const geocodeActiveOrderPoint = async (target) => {
    if (!activeOrder) {
      return;
    }

    const address = target === 'from' ? activeOrder.from.trim() : activeOrder.to.trim();
    if (address.length < 3) {
      setGeoStatus({ loading: false, error: 'Для геокодирования нужен адрес в выбранном заказе.' });
      return;
    }

    setGeoStatus({ loading: true, error: '' });

    try {
      const coords = await geocodeAddress(address);
      setOrders((prev) =>
        prev.map((order) =>
          order.id === activeOrder.id
            ? target === 'from'
              ? { ...order, fromCoords: coords }
              : { ...order, toCoords: coords }
            : order,
        ),
      );
      setBanner(target === 'from' ? `Точка A обновлена по адресу: ${coords[0]}, ${coords[1]}` : `Точка B обновлена по адресу: ${coords[0]}, ${coords[1]}`);
      setGeoStatus({ loading: false, error: '' });
    } catch (_error) {
      setGeoStatus({ loading: false, error: 'Геокодер не нашел этот адрес. Попробуй точнее.' });
    }
  };

  const validateForm = () => {
    if (form.from.trim().length < 5) {
      return 'Укажите корректный адрес отправления.';
    }
    if (form.to.trim().length < 5) {
      return 'Укажите корректный адрес доставки.';
    }
    if (!/^\+?[0-9\s()-]{10,}$/.test(form.phone)) {
      return 'Введите корректный телефон получателя.';
    }
    return '';
  };

  const handleCreateOrder = async (event) => {
    event.preventDefault();
    setFormError('');
    setBanner('');

    const validationError = validateForm();
    if (validationError) {
      setFormError(validationError);
      return;
    }

    const fromCoords = parseCoords(form.fromCoords) || DEFAULT_FROM_COORDS;
    const toCoords = parseCoords(form.toCoords) || DEFAULT_TO_COORDS;

    // Build order payload for the backend
    const orderPayload = {
      from_address: {
        city: 'Москва',
        street: form.from.trim().split(',')[0]?.trim() || form.from.trim(),
        building: '',
        apartment: '',
        comment: form.comment.trim(),
      },
      to_address: {
        city: 'Москва',
        street: form.to.trim().split(',')[0]?.trim() || form.to.trim(),
        building: '',
        apartment: '',
        comment: '',
      },
      weight: parseFloat(form.weight) || 0,
      distance_km: routeInfo.distanceKm || 0,
      user_id: '00000000-0000-0000-0000-000000000000',
    };

    try {
      const response = await fetch('/orders', {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
          ...(session.token ? { Authorization: `Bearer ${session.token}` } : {}),
        },
        body: JSON.stringify(orderPayload),
      });

      if (!response.ok) {
        const body = await response.text();
        throw new Error(body || `HTTP ${response.status}`);
      }

      const createdOrder = await response.json();

      const order = {
        id: createdOrder.id || `DLV-${Math.floor(10000 + Math.random() * 89999)}`,
        from: form.from.trim(),
        to: form.to.trim(),
        fromCoords: fromCoords,
        toCoords: toCoords,
        status: 'Создан',
        eta: `${Math.max(1, Math.round(routeInfo.durationMin || (12 + Math.random() * 20)))} мин`,
        price: createdOrder.price,
        weight: createdOrder.weight,
      };

      setOrders((prev) => [order, ...prev]);
      setActiveOrderId(order.id);
      setComputedPrice(null);
      setBanner(`Заказ ${order.id} создан. Стоимость: ${order.price}₽. Курьер назначается.`);
      setForm((prev) => ({
        ...prev,
        from: '',
        to: '',
        fromCoords: '',
        toCoords: '',
        phone: '',
        weight: '',
        comment: '',
      }));
    } catch (error) {
      // Fallback: create order locally if backend is unavailable
      const order = {
        id: `DLV-${Math.floor(10000 + Math.random() * 89999)}`,
        from: form.from.trim(),
        to: form.to.trim(),
        fromCoords: fromCoords,
        toCoords: toCoords,
        status: 'Создан',
        eta: `${Math.max(1, Math.round(routeInfo.durationMin || (12 + Math.random() * 20)))} мин`,
      };
      setOrders((prev) => [order, ...prev]);
      setActiveOrderId(order.id);
      setBanner(`Заказ ${order.id} создан (офлайн-режим).`);
      setForm((prev) => ({
        ...prev,
        from: '',
        to: '',
        fromCoords: '',
        toCoords: '',
        phone: '',
        weight: '',
        comment: '',
      }));
    }
  };

  const simulateStep = () => {
    if (!activeOrder) {
      return;
    }

    const currentIndex = STATUS_FLOW.indexOf(activeOrder.status);
    if (currentIndex >= STATUS_FLOW.length - 1) {
      setBanner(`Заказ ${activeOrder.id} уже завершен.`);
      return;
    }

    const nextStatus = STATUS_FLOW[currentIndex + 1];
    setOrders((prev) =>
      prev.map((order) =>
        order.id === activeOrder.id
          ? {
              ...order,
              status: nextStatus,
              eta: nextStatus === 'Доставлен' ? '0 мин' : order.eta,
            }
          : order,
      ),
    );
    setBanner(`Статус заказа ${activeOrder.id}: ${nextStatus}.`);
  };

  if (currentView === 'register') {
    return (
      <main className="page auth-page">
        <section className="auth-shell">
          <header className="auth-header">
            <p className="brand-eyebrow">Delivery Service</p>
            <h1>{authMode === 'register' ? 'Регистрация клиента' : 'Вход в аккаунт'}</h1>
            <p>{authMode === 'register' ? 'Сначала создаем профиль, затем сразу попадаем в заказ.' : 'Войди, чтобы открыть панель заказа.'}</p>
          </header>

          <form className="auth-form" onSubmit={handleAuthSubmit}>
            {authMode === 'register' ? (
              <>
                <label htmlFor="auth-name">Имя</label>
                <input id="auth-name" name="name" value={authForm.name} onChange={handleAuthChange} placeholder="Илья Гречин" />

                <label htmlFor="auth-phone">Телефон</label>
                <input id="auth-phone" name="phone" value={authForm.phone} onChange={handleAuthChange} placeholder="+79991234567" />

                <label htmlFor="auth-email">Email</label>
                <input id="auth-email" name="email" type="email" value={authForm.email} onChange={handleAuthChange} placeholder="mail@example.com" />

                <label htmlFor="auth-password">Пароль</label>
                <input id="auth-password" name="password" type="password" value={authForm.password} onChange={handleAuthChange} placeholder="Минимум 8 символов" />

                <label htmlFor="auth-confirmPassword">Подтверждение пароля</label>
                <input id="auth-confirmPassword" name="confirmPassword" type="password" value={authForm.confirmPassword} onChange={handleAuthChange} placeholder="Повтори пароль" />
              </>
            ) : (
              <>
                <label htmlFor="auth-login">Email или телефон</label>
                <input id="auth-login" name="login" value={authForm.login} onChange={handleAuthChange} placeholder="mail@example.com или +7999..." />

                <label htmlFor="auth-password-login">Пароль</label>
                <input id="auth-password-login" name="password" type="password" value={authForm.password} onChange={handleAuthChange} placeholder="Ваш пароль" />
              </>
            )}

            <button type="submit" className="submit-btn" disabled={authLoading}>
              {authLoading ? 'Подождите...' : authMode === 'register' ? 'Зарегистрироваться и войти' : 'Войти'}
            </button>

            <button
              type="button"
              className="link-btn"
              onClick={() => {
                setAuthError('');
                setAuthNotice('');
                setAuthMode((prev) => (prev === 'register' ? 'login' : 'register'));
              }}
            >
              {authMode === 'register' ? 'Уже есть аккаунт? Войти' : 'Нет аккаунта? Зарегистрироваться'}
            </button>

            {authError && <p className="api-error">{authError}</p>}
            {authNotice && <p className="success">{authNotice}</p>}
          </form>
        </section>
      </main>
    );
  }

  if (currentView === 'profile') {
    return (
      <main className="page">
        <section className="dashboard profile-dashboard">
          <header className="topline profile-topline">
            <div>
              <p className="brand-eyebrow">Delivery Service</p>
              <h1>Профиль пользователя</h1>
              <p className="profile-lead">Здесь можно быстро обновить данные, посмотреть активность и вернуться к заказам.</p>
            </div>
            <div className="topline-right">
              <div className="chip-row">
                <span>{orders.length} заказов</span>
                <span>{inProgressOrders} в работе</span>
                <span>{completedOrders} доставлено</span>
              </div>
              <div className="profile-actions">
                <button type="button" className="profile-btn" onClick={() => setCurrentView('order')}>
                  К заказам
                </button>
                <button type="button" className="profile-btn ghost-btn" onClick={handleLogout}>
                  Выйти
                </button>
              </div>
            </div>
          </header>

          <div className="profile-grid">
            <article className="profile-summary card-shell">
              <div className="profile-avatar">
                <span>{(profileForm.name || session.profile.name || 'П').trim().charAt(0).toUpperCase()}</span>
              </div>
              <div>
                <p className="brand-eyebrow">Аккаунт</p>
                <h2>{profileForm.name || 'Профиль без имени'}</h2>
                <p className="profile-muted">{profileForm.email || 'email не указан'}</p>
              </div>

              <div className="profile-stats">
                <div className="stat-card">
                  <strong>{orders.length}</strong>
                  <span>Всего заказов</span>
                </div>
                <div className="stat-card">
                  <strong>{activeOrder ? 1 : 0}</strong>
                  <span>Активный заказ</span>
                </div>
                <div className="stat-card">
                  <strong>{completedOrders}</strong>
                  <span>Доставлено</span>
                </div>
              </div>

              {activeOrder && (
                <div className="profile-spotlight">
                  <p className="brand-eyebrow">Последний выбранный заказ</p>
                  <strong>{activeOrder.id}</strong>
                  <span>{activeOrder.from} → {activeOrder.to}</span>
                </div>
              )}
            </article>

            <article className="profile-form-card card-shell">
              <h2>Данные профиля</h2>
              <p className="panel-caption">Эти данные подставляются в новые заказы и сохраняются в браузере.</p>

              <form className="profile-form" onSubmit={handleSaveProfile}>
                <div className="field-row">
                  <div className="field">
                    <label htmlFor="profile-name">Имя</label>
                    <input id="profile-name" name="name" value={profileForm.name} onChange={handleProfileChange} placeholder="Ваше имя" />
                  </div>

                  <div className="field">
                    <label htmlFor="profile-phone">Телефон</label>
                    <input id="profile-phone" name="phone" value={profileForm.phone} onChange={handleProfileChange} placeholder="+7999..." />
                  </div>
                </div>

                <div className="field">
                  <label htmlFor="profile-email">Email</label>
                  <input id="profile-email" name="email" type="email" value={profileForm.email} onChange={handleProfileChange} placeholder="mail@example.com" />
                </div>

                <div className="profile-actions stacked">
                  <button type="submit" className="submit-btn">Сохранить профиль</button>
                  <button type="button" className="submit-btn ghost" onClick={() => setCurrentView('order')}>Вернуться к заказам</button>
                </div>

                {profileNotice && <p className="success">{profileNotice}</p>}
              </form>
            </article>

            <article className="profile-orders card-shell">
              <div className="section-heading">
                <div>
                  <p className="brand-eyebrow">Недавние заказы</p>
                  <h2>Последние доставки</h2>
                </div>
                <button type="button" className="link-btn" onClick={() => setCurrentView('order')}>
                  Открыть панель заказов
                </button>
              </div>

              <ul className="orders-list compact">
                {recentOrders.map((order) => (
                  <li key={`profile-${order.id}`}>
                    <button type="button" className="order-item compact-item" onClick={() => { setActiveOrderId(order.id); setCurrentView('order'); }}>
                      <div>
                        <strong>{order.id}</strong>
                        <p>{order.from}</p>
                        <p>{order.to}</p>
                      </div>
                      <div className="order-aside">
                        <span>{order.status}</span>
                        <small>{order.eta}</small>
                      </div>
                    </button>
                  </li>
                ))}
              </ul>
            </article>
          </div>
        </section>
      </main>
    );
  }

  return (
    <main className="page">
      <section className="dashboard">
        <header className="topline">
          <div>
            <p className="brand-eyebrow">Delivery Service</p>
            <h1>Заказы и доставка</h1>
            <p className="profile-lead">Список заказов, создание новой доставки и карта маршрута в одном экране.</p>
          </div>
          <div className="topline-right">
            <div className="chip-row">
              <span>{orders.length} заказов</span>
              <span>{inProgressOrders} в работе</span>
              <span>Онлайн-карта</span>
            </div>
            <button type="button" className="profile-btn" onClick={() => setCurrentView('profile')}>
              Профиль
            </button>
          </div>
        </header>

        <div className={mapExpanded ? 'layout-grid map-expanded' : 'layout-grid'}>
          <section className="order-panel card-shell">
            <h2>Новый заказ</h2>
            <form className="order-form" onSubmit={handleCreateOrder}>
              <div className="field">
                <label htmlFor="from">Откуда забрать</label>
                <input
                  id="from"
                  name="from"
                  type="text"
                  placeholder="ул. Пушкина, 12"
                  value={form.from}
                  onChange={handleChange}
                />
                <button type="button" className="geo-btn" onClick={() => geocodeFormPoint('from')} disabled={geoStatus.loading}>
                  {geoStatus.loading ? 'Поиск...' : 'Определить точку A по адресу'}
                </button>
                <button type="button" className="geo-btn" onClick={() => fetchAddressSuggestions('from')} disabled={geoStatus.loading}>
                  {geoStatus.loading ? 'Поиск...' : 'Показать варианты A'}
                </button>
                {addressSuggestions.from.length > 0 && (
                  <ul className="suggestion-list">
                    {addressSuggestions.from.map((item) => (
                      <li key={`from-${item.display_name}-${item.lat}-${item.lon}`}>
                        <button type="button" className="suggestion-item" onClick={() => applySuggestion('from', item)}>
                          <strong>{item.display_name}</strong>
                          <small>{item.lat}, {item.lon}</small>
                        </button>
                      </li>
                    ))}
                  </ul>
                )}
                <p className="field-hint">Координаты точки A определяются автоматически.</p>
              </div>

              <div className="field">
                <label htmlFor="to">Куда доставить</label>
                <input
                  id="to"
                  name="to"
                  type="text"
                  placeholder="Невский проспект, 8"
                  value={form.to}
                  onChange={handleChange}
                />
                <button type="button" className="geo-btn" onClick={() => geocodeFormPoint('to')} disabled={geoStatus.loading}>
                  {geoStatus.loading ? 'Поиск...' : 'Определить точку B по адресу'}
                </button>
                <button type="button" className="geo-btn" onClick={() => fetchAddressSuggestions('to')} disabled={geoStatus.loading}>
                  {geoStatus.loading ? 'Поиск...' : 'Показать варианты B'}
                </button>
                {addressSuggestions.to.length > 0 && (
                  <ul className="suggestion-list">
                    {addressSuggestions.to.map((item) => (
                      <li key={`to-${item.display_name}-${item.lat}-${item.lon}`}>
                        <button type="button" className="suggestion-item" onClick={() => applySuggestion('to', item)}>
                          <strong>{item.display_name}</strong>
                          <small>{item.lat}, {item.lon}</small>
                        </button>
                      </li>
                    ))}
                  </ul>
                )}
                <p className="field-hint">Координаты точки B определяются автоматически.</p>
              </div>

              <div className="field-row">
                <div className="field">
                  <label htmlFor="phone">Телефон получателя</label>
                  <input
                    id="phone"
                    name="phone"
                    type="tel"
                    placeholder="+7 (999) 777-11-22"
                    value={form.phone}
                    onChange={handleChange}
                  />
                </div>
                <div className="field">
                  <label htmlFor="weight">Вес, кг</label>
                  <input
                    id="weight"
                    name="weight"
                    type="number"
                    min="0"
                    step="0.1"
                    placeholder="1.5"
                    value={form.weight}
                    onChange={handleChange}
                  />
                </div>
              </div>

              {routeInfo.distanceKm !== null && (
                <div className="price-preview">
                  <p>
                    Дистанция: <strong>{routeInfo.distanceKm} км</strong>
                    {form.weight && parseFloat(form.weight) > 0 && (
                      <>
                        {' '}
                        · Вес: <strong>{form.weight} кг</strong>
                      </>
                    )}
                  </p>
                  <p className="price-value">
                    Стоимость доставки: <strong>{computedPrice !== null ? computedPrice + '₽' : 'рассчитывается...'}</strong>
                  </p>
                </div>
              )}

              <div className="field-row">
                <div className="field">
                  <label htmlFor="pickupTime">Время забора</label>
                  <select
                    id="pickupTime"
                    name="pickupTime"
                    value={form.pickupTime}
                    onChange={handleChange}
                  >
                    <option>Как можно скорее</option>
                    <option>Через 30 минут</option>
                    <option>Через 1 час</option>
                    <option>Сегодня вечером</option>
                  </select>
                </div>
                <div className="field">
                  <label htmlFor="payment">Оплата</label>
                  <select
                    id="payment"
                    name="payment"
                    value={form.payment}
                    onChange={handleChange}
                  >
                    <option value="card">Карта</option>
                    <option value="cash">Наличные</option>
                    <option value="invoice">Безнал для юр.лица</option>
                  </select>
                </div>
              </div>

              <div className="field">
                <label htmlFor="comment">Комментарий для курьера</label>
                <textarea
                  id="comment"
                  name="comment"
                  rows="3"
                  placeholder="Код домофона, этаж, детали передачи"
                  value={form.comment}
                  onChange={handleChange}
                />
              </div>

              <button type="submit" className="submit-btn">
                Оформить доставку
              </button>

              {formError && <p className="api-error">{formError}</p>}
              {geoStatus.error && <p className="api-error">{geoStatus.error}</p>}
              {banner && <p className="success">{banner}</p>}
            </form>
          </section>

          <section className={mapExpanded ? 'map-panel card-shell map-panel-expanded' : 'map-panel card-shell'}>
            <div className="map-header">
              <h2>Карта маршрута</h2>
              <span>{activeOrder?.eta || 'нет данных'}</span>
            </div>
            <div className="map-tools">
              <button
                type="button"
                className={mapEditTarget === 'from' ? 'map-target-btn active' : 'map-target-btn'}
                onClick={() => setMapEditTarget('from')}
              >
                Ставить точку A
              </button>
              <button
                type="button"
                className={mapEditTarget === 'to' ? 'map-target-btn active' : 'map-target-btn'}
                onClick={() => setMapEditTarget('to')}
              >
                Ставить точку B
              </button>
              <button type="button" className="map-target-btn" onClick={() => setMapExpanded((prev) => !prev)}>
                {mapExpanded ? 'Свернуть карту' : 'Развернуть карту'}
              </button>
              <button type="button" className="map-target-btn" onClick={() => geocodeActiveOrderPoint('from')} disabled={geoStatus.loading}>
                A из адреса заказа
              </button>
              <button type="button" className="map-target-btn" onClick={() => geocodeActiveOrderPoint('to')} disabled={geoStatus.loading}>
                B из адреса заказа
              </button>
            </div>
            <div className="map-canvas interactive">
              <MapContainer center={mapCenter} zoom={11} scrollWheelZoom className="leaflet-map">
                <TileLayer
                  attribution='&copy; OpenStreetMap contributors'
                  url="https://{s}.tile.openstreetmap.org/{z}/{x}/{y}.png"
                />
                <MapClickHandler onPick={handleMapClick} />

                {activeOrder?.fromCoords && (
                  <CircleMarker center={activeOrder.fromCoords} radius={8} pathOptions={{ color: '#2f7fd8' }}>
                    <Tooltip direction="top" permanent>
                      A
                    </Tooltip>
                  </CircleMarker>
                )}

                {activeOrder?.toCoords && (
                  <CircleMarker center={activeOrder.toCoords} radius={8} pathOptions={{ color: '#e55a2b' }}>
                    <Tooltip direction="top" permanent>
                      B
                    </Tooltip>
                  </CircleMarker>
                )}

                {routeInfo.geometry.length > 1 && (
                  <Polyline positions={routeInfo.geometry} pathOptions={{ color: '#1d5faa', weight: 5 }} />
                )}
              </MapContainer>
            </div>
            <div className="map-meta">
              <p>Клик по карте ставит {mapEditTarget === 'from' ? 'точку A' : 'точку B'}.</p>
              <p>
                <strong>Откуда:</strong> {activeOrder?.from || '—'}
              </p>
              <p>
                <strong>Куда:</strong> {activeOrder?.to || '—'}
              </p>
              <p>
                <strong>Дистанция:</strong>{' '}
                {routeInfo.loading
                  ? 'Считаем...'
                  : routeInfo.distanceKm !== null
                    ? `${routeInfo.distanceKm} км`
                    : '—'}
              </p>
              <p>
                <strong>В пути:</strong>{' '}
                {routeInfo.loading
                  ? 'Считаем...'
                  : routeInfo.durationMin !== null
                    ? `${routeInfo.durationMin} мин`
                    : '—'}
              </p>
              {activeOrder?.price && (
                <p>
                  <strong>Стоимость:</strong>{' '}
                  <span style={{ color: 'var(--accent)', fontWeight: 700 }}>
                    {activeOrder.price}₽
                  </span>
                </p>
              )}
              {activeOrder?.weight && (
                <p>
                  <strong>Вес:</strong> {activeOrder.weight} кг
                </p>
              )}
              {routeInfo.error && <p className="api-error">{routeInfo.error}</p>}
            </div>
          </section>
        </div>

        <section className="bottom-grid">
          <article className="orders-panel card-shell">
            <h2>Активные заказы</h2>
            <ul className="orders-list">
              {orders.map((order) => (
                <li key={order.id}>
                  <button
                    type="button"
                    className={order.id === activeOrderId ? 'order-item active' : 'order-item'}
                    onClick={() => setActiveOrderId(order.id)}
                  >
                    <div>
                      <strong>{order.id}</strong>
                      <p>{order.from}</p>
                      <p>{order.to}</p>
                    </div>
                    <div className="order-aside">
                      <span>{order.status}</span>
                      <small>{order.eta}</small>
                    </div>
                  </button>
                </li>
              ))}
            </ul>
          </article>

          <article className="track-panel card-shell">
            <h2>Трекинг заказа {activeOrder?.id || ''}</h2>
            <ol className="track-steps">
              {STATUS_FLOW.map((step, index) => (
                <li key={step} className={index <= activeStep ? 'done' : ''}>
                  {step}
                </li>
              ))}
            </ol>
            <button type="button" className="submit-btn ghost" onClick={simulateStep}>
              Обновить статус
            </button>
          </article>
        </section>
      </section>
    </main>
  );
}
