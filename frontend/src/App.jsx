import { useEffect, useMemo, useState } from 'react';
import { CircleMarker, MapContainer, Polyline, TileLayer, Tooltip, useMapEvents } from 'react-leaflet';

const STATUS_FLOW = ['Создан', 'Курьер в пути', 'Забран у отправителя', 'Доставлен'];

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
  const mapCenter = useMemo(() => {
    const from = activeOrder?.fromCoords || DEFAULT_FROM_COORDS;
    const to = activeOrder?.toCoords || DEFAULT_TO_COORDS;
    return [Number(((from[0] + to[0]) / 2).toFixed(6)), Number(((from[1] + to[1]) / 2).toFixed(6))];
  }, [activeOrder]);

  useEffect(() => {
    if (!activeOrder) {
      return;
    }

    const fromCoords = activeOrder.fromCoords || DEFAULT_FROM_COORDS;
    const toCoords = activeOrder.toCoords || DEFAULT_TO_COORDS;
    const controller = new AbortController();

    const loadRoute = async () => {
      setRouteInfo({ loading: true, distanceKm: null, durationMin: null, geometry: [], error: '' });

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
          throw new Error(`HTTP ${response.status}`);
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
      } catch (error) {
        if (error.name === 'AbortError') {
          return;
        }
        setRouteInfo({
          loading: false,
          distanceKm: null,
          durationMin: null,
          geometry: [],
          error: 'Маршрут недоступен. Проверь /api/route и контейнеры auth-service + osrm.',
        });
      }
    };

    loadRoute();

    return () => {
      controller.abort();
    };
  }, [activeOrder]);

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

  const handleCreateOrder = (event) => {
    event.preventDefault();
    setFormError('');
    setBanner('');

    const validationError = validateForm();
    if (validationError) {
      setFormError(validationError);
      return;
    }

    const order = {
      id: `DLV-${Math.floor(10000 + Math.random() * 89999)}`,
      from: form.from.trim(),
      to: form.to.trim(),
      fromCoords: parseCoords(form.fromCoords) || DEFAULT_FROM_COORDS,
      toCoords: parseCoords(form.toCoords) || DEFAULT_TO_COORDS,
      status: 'Создан',
      eta: `${12 + Math.floor(Math.random() * 20)} мин`,
    };

    setOrders((prev) => [order, ...prev]);
    setActiveOrderId(order.id);
    setBanner(`Заказ ${order.id} создан. Курьер назначается.`);
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

  return (
    <main className="page">
      <section className="dashboard">
        <header className="topline">
          <div>
            <p className="brand-eyebrow">Delivery Service</p>
            <h1>Главная панель заказов</h1>
          </div>
          <div className="chip-row">
            <span>Быстрый заказ</span>
            <span>Онлайн-карта</span>
            <span>Статусы в реальном времени</span>
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

              <div className="field-row">
                <div className="field">
                  <label htmlFor="fromCoords">Координаты точки A (lat,lon)</label>
                  <input
                    id="fromCoords"
                    name="fromCoords"
                    type="text"
                    placeholder="55.7558,37.6176"
                    value={form.fromCoords}
                    onChange={handleChange}
                  />
                </div>
                <div className="field">
                  <label htmlFor="toCoords">Координаты точки B (lat,lon)</label>
                  <input
                    id="toCoords"
                    name="toCoords"
                    type="text"
                    placeholder="55.7060,37.5895"
                    value={form.toCoords}
                    onChange={handleChange}
                  />
                </div>
              </div>

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
