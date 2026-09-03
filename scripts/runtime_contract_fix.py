from pathlib import Path


def read(path):
    return Path(path).read_text()


def write(path, text):
    Path(path).write_text(text)


def replace_once(path, old, new):
    text = read(path)
    if old not in text:
        raise SystemExit(f'expected snippet not found in {path}: {old[:100]!r}')
    write(path, text.replace(old, new, 1))


def replace_between(path, start, end, replacement):
    text = read(path)
    a = text.find(start)
    if a < 0:
        raise SystemExit(f'start marker not found in {path}: {start!r}')
    b = text.find(end, a)
    if b < 0:
        raise SystemExit(f'end marker not found in {path}: {end!r}')
    write(path, text[:a] + replacement.rstrip() + '\n\n' + text[b:])


replace_once(
    'backend/order-service/internal/storage/order_repository.go',
    'ListByUser(ctx context.Context, userID, status string, page, limit int, sort string) ([]models.Order, int, error)\n',
    'ListByUser(ctx context.Context, userID, status string, page, limit int, sort string) ([]models.Order, int, error)\n\tListForCourier(ctx context.Context, userID, status string, page, limit int, sort string) ([]models.Order, int, error)\n',
)

repo_path = 'backend/order-service/internal/storage/postgres_order_repository.go'
repo_text = read(repo_path)
marker = 'func (r *PostgresOrderRepository) UpdateStatus'
idx = repo_text.find(marker)
if idx < 0:
    raise SystemExit('UpdateStatus marker not found')
method = r'''func (r *PostgresOrderRepository) ListForCourier(ctx context.Context, userID, status string, page, limit int, sort string) ([]models.Order, int, error) {
\taccessClause := `(
\t\to.status = 'created'
\t\tOR EXISTS (
\t\t\tSELECT 1
\t\t\tFROM assignments a
\t\t\tJOIN couriers c ON c.id = a.courier_id
\t\t\tWHERE a.order_id = o.id AND c.user_id = $1
\t\t)
\t)`

\tcountQuery := `SELECT COUNT(*) FROM orders o WHERE ` + accessClause
\tcountArgs := []any{userID}
\tif status != "" {
\t\tcountQuery += ` AND o.status = $2`
\t\tcountArgs = append(countArgs, status)
\t}
\tvar total int
\tif err := r.pool.QueryRow(ctx, countQuery, countArgs...).Scan(&total); err != nil {
\t\treturn nil, 0, fmt.Errorf("count courier orders: %w", err)
\t}

\torderBy := "o.created_at DESC"
\tswitch sort {
\tcase "price_asc":
\t\torderBy = "o.price ASC, o.created_at DESC"
\tcase "price_desc":
\t\torderBy = "o.price DESC, o.created_at DESC"
\t}

\targs := []any{userID}
\tquery := `
\t\tSELECT o.id, o.user_id, o.from_address, o.to_address, o.from_coords, o.to_coords,
\t\t       o.weight, o.distance_km, o.price, o.status, o.created_at, o.updated_at
\t\tFROM orders o
\t\tWHERE ` + accessClause
\tif status != "" {
\t\tquery += ` AND o.status = $2`
\t\targs = append(args, status)
\t}
\tquery += " ORDER BY " + orderBy
\targs = append(args, limit, (page-1)*limit)
\tquery += fmt.Sprintf(" LIMIT $%d OFFSET $%d", len(args)-1, len(args))

\torders, err := r.queryOrders(ctx, query, args...)
\tif err != nil {
\t\treturn nil, 0, err
\t}
\treturn orders, total, nil
}

'''
write(repo_path, repo_text[:idx] + method + repo_text[idx:])

handler_path = 'backend/order-service/internal/handlers/handler.go'
replace_once(
    handler_path,
    '''\tuserID, ok := middleware.UserID(r.Context())
\tif !ok {
\t\tjsonError(w, http.StatusUnauthorized, "unauthorized")
\t\treturn
\t}
\tvar req CreateOrderRequest
''',
    '''\tuserID, ok := middleware.UserID(r.Context())
\tif !ok {
\t\tjsonError(w, http.StatusUnauthorized, "unauthorized")
\t\treturn
\t}
\trole, _ := middleware.Role(r.Context())
\tif role != "client" {
\t\tjsonError(w, http.StatusForbidden, "only clients can create orders")
\t\treturn
\t}
\tvar req CreateOrderRequest
''',
)

replace_between(
    handler_path,
    'func (h *OrdersHandler) ListOrders',
    'func validateOrderRequest',
    r'''func (h *OrdersHandler) ListOrders(w http.ResponseWriter, r *http.Request) {
\tuserID, ok := middleware.UserID(r.Context())
\tif !ok {
\t\tjsonError(w, http.StatusUnauthorized, "unauthorized")
\t\treturn
\t}
\trole, ok := middleware.Role(r.Context())
\tif !ok {
\t\tjsonError(w, http.StatusForbidden, "role is required")
\t\treturn
\t}

\tpage := parseBoundedInt(r.URL.Query().Get("page"), 1, 1, 1_000_000)
\tlimit := parseBoundedInt(r.URL.Query().Get("limit"), 20, 1, 100)
\tstatus := strings.TrimSpace(r.URL.Query().Get("status"))
\tif status != "" && !validOrderStatus(status) {
\t\tjsonError(w, http.StatusBadRequest, "invalid status")
\t\treturn
\t}
\tsort := strings.TrimSpace(r.URL.Query().Get("sort"))
\tif sort != "" && sort != "price_asc" && sort != "price_desc" {
\t\tjsonError(w, http.StatusBadRequest, "invalid sort")
\t\treturn
\t}

\tvar (
\t\torders []models.Order
\t\ttotal  int
\t\terr    error
\t)
\tswitch role {
\tcase "client":
\t\torders, total, err = h.repo.ListByUser(r.Context(), userID, status, page, limit, sort)
\tcase "courier":
\t\torders, total, err = h.repo.ListForCourier(r.Context(), userID, status, page, limit, sort)
\tdefault:
\t\tjsonError(w, http.StatusForbidden, "unsupported role")
\t\treturn
\t}
\tif err != nil {
\t\tjsonError(w, http.StatusInternalServerError, "failed to list orders")
\t\treturn
\t}
\tjsonResponse(w, http.StatusOK, map[string]any{"orders": orders, "total": total, "page": page, "limit": limit})
}''',
)

courier_path = 'backend/order-service/internal/handlers/courier_handler.go'
text = read(courier_path)
register_start = text.find('func (h *CourierHandler) RegisterCourier')
register_end = text.find('func (h *CourierHandler) GetCourierByEmail', register_start)
register = text[register_start:register_end]
old_auth = '''\tuserID, ok := middleware.UserID(r.Context())
\tif !ok {
\t\tjsonError(w, http.StatusUnauthorized, "unauthorized")
\t\treturn
\t}
'''
new_auth = old_auth + '''\trole, _ := middleware.Role(r.Context())
\tif role != "courier" {
\t\tjsonError(w, http.StatusForbidden, "courier role is required")
\t\treturn
\t}
'''
if old_auth not in register:
    raise SystemExit('RegisterCourier auth snippet not found')
register = register.replace(old_auth, new_auth, 1)
text = text[:register_start] + register + text[register_end:]
write(courier_path, text)

text = read(courier_path)
lookup_start = text.find('func (h *CourierHandler) GetCourierByEmail')
lookup_end = text.find('func (h *CourierHandler) ToggleAvailability', lookup_start)
lookup = text[lookup_start:lookup_end]
if old_auth not in lookup:
    raise SystemExit('GetCourierByEmail auth snippet not found')
lookup = lookup.replace(old_auth, new_auth, 1)
text = text[:lookup_start] + lookup + text[lookup_end:]
write(courier_path, text)

replace_between(
    courier_path,
    'func (h *CourierHandler) AssignOrder',
    'func (h *CourierHandler) GetActiveOrder',
    r'''func (h *CourierHandler) AssignOrder(w http.ResponseWriter, r *http.Request) {
\tuserID, ok := middleware.UserID(r.Context())
\tif !ok {
\t\tjsonError(w, http.StatusUnauthorized, "unauthorized")
\t\treturn
\t}
\trole, ok := middleware.Role(r.Context())
\tif !ok || (role != "client" && role != "courier") {
\t\tjsonError(w, http.StatusForbidden, "unsupported role")
\t\treturn
\t}

\torderID := strings.TrimSpace(r.PathValue("orderId"))
\tif _, err := uuid.Parse(orderID); err != nil {
\t\tjsonError(w, http.StatusBadRequest, "invalid order_id")
\t\treturn
\t}
\torder, err := h.orderRepo.GetByID(r.Context(), orderID)
\tif errors.Is(err, pgx.ErrNoRows) {
\t\tjsonError(w, http.StatusNotFound, "order not found")
\t\treturn
\t}
\tif err != nil {
\t\tjsonError(w, http.StatusInternalServerError, "failed to get order")
\t\treturn
\t}

\tvar req struct {
\t\tCourierID string `json:"courier_id"`
\t\tMode      string `json:"mode"`
\t}
\tif err := decodeJSON(w, r, &req); err != nil {
\t\tjsonError(w, http.StatusBadRequest, "invalid request body")
\t\treturn
\t}
\tmode := strings.ToLower(strings.TrimSpace(req.Mode))
\tif mode == "" {
\t\tmode = "auto"
\t}

\tvar courierID string
\tswitch mode {
\tcase "auto":
\t\tif role != "client" {
\t\t\tjsonError(w, http.StatusForbidden, "auto assignment is available to clients only")
\t\t\treturn
\t\t}
\t\tif order.UserID != userID {
\t\t\tjsonError(w, http.StatusNotFound, "order not found")
\t\t\treturn
\t\t}
\t\tcouriers, err := h.courierRepo.FindAvailable(r.Context())
\t\tif err != nil {
\t\t\tjsonError(w, http.StatusInternalServerError, "failed to find available couriers")
\t\t\treturn
\t\t}
\t\tif len(couriers) == 0 {
\t\t\tjsonError(w, http.StatusNotFound, "no available couriers")
\t\t\treturn
\t\t}
\t\tcourierID = couriers[0].ID
\tcase "manual":
\t\tif role != "courier" {
\t\t\tjsonError(w, http.StatusForbidden, "manual assignment is available to couriers only")
\t\t\treturn
\t\t}
\t\townedCourier, err := h.courierRepo.GetByUserID(r.Context(), userID)
\t\tif errors.Is(err, pgx.ErrNoRows) {
\t\t\tjsonError(w, http.StatusNotFound, "courier profile not found")
\t\t\treturn
\t\t}
\t\tif err != nil {
\t\t\tjsonError(w, http.StatusInternalServerError, "failed to get courier profile")
\t\t\treturn
\t\t}
\t\trequestedID := strings.TrimSpace(req.CourierID)
\t\tif requestedID != "" && requestedID != ownedCourier.ID {
\t\t\tjsonError(w, http.StatusForbidden, "cannot assign an order to another courier")
\t\t\treturn
\t\t}
\t\tcourierID = ownedCourier.ID
\tdefault:
\t\tjsonError(w, http.StatusBadRequest, "mode must be auto or manual")
\t\treturn
\t}

\tcourier, err := h.courierRepo.GetByID(r.Context(), courierID)
\tif errors.Is(err, pgx.ErrNoRows) {
\t\tjsonError(w, http.StatusNotFound, "courier not found")
\t\treturn
\t}
\tif err != nil {
\t\tjsonError(w, http.StatusInternalServerError, "failed to get courier")
\t\treturn
\t}
\tetaDuration := computeETADuration(courier.TransportType, order.DistanceKm)
\tassignment := models.NewAssignment(orderID, courierID, etaDuration)
\tif err := h.assignmentRepo.Assign(r.Context(), assignment); err != nil {
\t\tswitch {
\t\tcase errors.Is(err, storage.ErrCourierNotFound):
\t\t\tjsonError(w, http.StatusNotFound, "courier not found")
\t\tcase errors.Is(err, storage.ErrOrderNotFound):
\t\t\tjsonError(w, http.StatusNotFound, "order not found")
\t\tcase errors.Is(err, storage.ErrCourierBusy), errors.Is(err, storage.ErrCourierUnavailable),
\t\t\terrors.Is(err, storage.ErrOrderAlreadyAssigned), errors.Is(err, storage.ErrOrderNotAssignable):
\t\t\tjsonError(w, http.StatusConflict, err.Error())
\t\tdefault:
\t\t\tobservability.Logger().Error("courier_assign_failed", "error", err, "order_id", orderID, "courier_id", courierID)
\t\t\tjsonError(w, http.StatusInternalServerError, "failed to assign order")
\t\t}
\t\treturn
\t}

\tobservability.Logger().Info("courier_assigned", "order_id", orderID, "courier_id", courierID, "mode", mode)
\tobservability.Stats().ObserveBusiness("courier_assign", "success")
\tjsonResponse(w, http.StatusOK, map[string]any{"courier_id": courierID, "eta": computeETA(courier.TransportType, order.DistanceKm)})
}''',
)

replace_between(
    courier_path,
    'func (h *CourierHandler) loadOwnedCourier',
    'func writeCourier',
    r'''func (h *CourierHandler) loadOwnedCourier(w http.ResponseWriter, r *http.Request) (*models.Courier, bool) {
\tuserID, ok := middleware.UserID(r.Context())
\tif !ok {
\t\tjsonError(w, http.StatusUnauthorized, "unauthorized")
\t\treturn nil, false
\t}
\trole, _ := middleware.Role(r.Context())
\tif role != "courier" {
\t\tjsonError(w, http.StatusForbidden, "courier role is required")
\t\treturn nil, false
\t}
\tcourier, err := h.courierRepo.GetByUserID(r.Context(), userID)
\tif errors.Is(err, pgx.ErrNoRows) {
\t\tjsonError(w, http.StatusNotFound, "courier profile not found")
\t\treturn nil, false
\t}
\tif err != nil {
\t\tobservability.Logger().Error("courier_owner_lookup_failed", "error", err, "user_id", userID)
\t\tjsonError(w, http.StatusInternalServerError, "failed to get courier profile")
\t\treturn nil, false
\t}
\treturn courier, true
}''',
)

test_path = 'backend/order-service/internal/handlers/courier_handler_test.go'
replace_once(
    test_path,
    '''func withUser(req *http.Request, userID string) *http.Request {
\treturn req.WithContext(middleware.WithIdentity(req.Context(), userID, "client"))
}
''',
    '''func withRole(req *http.Request, userID, role string) *http.Request {
\treturn req.WithContext(middleware.WithIdentity(req.Context(), userID, role))
}

func withUser(req *http.Request, userID string) *http.Request {
\treturn withRole(req, userID, "client")
}

func withCourier(req *http.Request, userID string) *http.Request {
\treturn withRole(req, userID, "courier")
}
''',
)
for needle in [
    'req := withUser(httptest.NewRequest(http.MethodPost, "/api/v1/couriers/register"',
    'req := withUser(httptest.NewRequest(http.MethodPost, "/api/v1/couriers/availability"',
    'req := withUser(httptest.NewRequest(http.MethodPost, "/api/v1/couriers/location"',
    'req := withUser(httptest.NewRequest(http.MethodPatch, "/api/v1/orders/"+orderID+"/status"',
]:
    replace_once(test_path, needle, needle.replace('withUser', 'withCourier', 1))
replace_once(
    test_path,
    'req := withUser(httptest.NewRequest(http.MethodPost, "/api/v1/orders/"+orderID+"/assign", strings.NewReader(`{"courier_id":"`+courierID+`","mode":"manual"}`)), "42")',
    'req := withCourier(httptest.NewRequest(http.MethodPost, "/api/v1/orders/"+orderID+"/assign", strings.NewReader(`{"courier_id":"`+courierID+`","mode":"manual"}`)), "77")',
)

app = 'frontend/src/App.jsx'
replace_once(
    app,
    "const formatOrderStatus = (status) => ORDER_STATUS_LABELS[status] || status || '—';\n",
    """const formatOrderStatus = (status) => ORDER_STATUS_LABELS[status] || status || '—';

const extractOrders = (payload) => {
  if (Array.isArray(payload)) {
    return payload;
  }
  return Array.isArray(payload?.orders) ? payload.orders : [];
};

const normalizeCoords = (value) => {
  if (Array.isArray(value) && value.length >= 2) {
    return value;
  }
  if (Number.isFinite(value?.latitude) && Number.isFinite(value?.longitude)) {
    return [value.latitude, value.longitude];
  }
  return null;
};

const authHeaders = (token, json = false) => ({
  ...(json ? { 'Content-Type': 'application/json' } : {}),
  ...(token ? { Authorization: `Bearer ${token}` } : {}),
});
""",
)
replace_once(
    app,
    '''  fromCoords: order.fromCoords || order.from_coords || null,
  toCoords: order.toCoords || order.to_coords || null,
''',
    '''  fromCoords: normalizeCoords(order.fromCoords || order.from_coords),
  toCoords: normalizeCoords(order.toCoords || order.to_coords),
''',
)
replace_once(app, "const response = await fetch('/api/v1/orders', { signal: controller.signal });", "const response = await fetch('/api/v1/orders', { signal: controller.signal, headers: authHeaders(session.token) });")
replace_once(
    app,
    '''        const data = await response.json();
        if (!Array.isArray(data) || data.length === 0) {
          return;
        }

        const previousOrders = new Map(ordersRef.current.map((order) => [order.id, order]));
        const mappedOrders = data.map((backendOrder) => {
''',
    '''        const data = await response.json();
        const backendOrders = extractOrders(data);
        if (backendOrders.length === 0) {
          setOrders([]);
          return;
        }

        const previousOrders = new Map(ordersRef.current.map((order) => [order.id, order]));
        const mappedOrders = backendOrders.map((backendOrder) => {
''',
)
replace_once(
    app,
    '''    loadBackendOrders();
    const intervalId = setInterval(loadBackendOrders, 15000);

    return () => {
      controller.abort();
      clearInterval(intervalId);
    };
  }, []);
''',
    '''    if (session.token) {
      loadBackendOrders();
    }
    const intervalId = session.token ? setInterval(loadBackendOrders, 15000) : null;

    return () => {
      controller.abort();
      if (intervalId) {
        clearInterval(intervalId);
      }
    };
  }, [session.token]);
''',
)
replace_once(app, 'const response = await fetch(`/api/v1/couriers/by-email?${params.toString()}`);', 'const response = await fetch(`/api/v1/couriers/by-email?${params.toString()}`, { headers: authHeaders(session.token) });')
replace_once(app, "const response = await fetch('/api/v1/orders');", "const response = await fetch('/api/v1/orders', { headers: authHeaders(session.token) });")
replace_once(
    app,
    '''      const data = await response.json();
      if (!Array.isArray(data) || data.length === 0) {
        return ordersRef.current;
      }

      const previousOrders = new Map(ordersRef.current.map((order) => [order.id, order]));
      const mappedOrders = data.map((backendOrder) => {
''',
    '''      const data = await response.json();
      const backendOrders = extractOrders(data);
      if (backendOrders.length === 0) {
        setOrders([]);
        return [];
      }

      const previousOrders = new Map(ordersRef.current.map((order) => [order.id, order]));
      const mappedOrders = backendOrders.map((backendOrder) => {
''',
)

insert_marker = '  const handleAuthSubmit = async (event) => {'
text = read(app)
idx = text.find(insert_marker)
if idx < 0:
    raise SystemExit('handleAuthSubmit marker not found')
helper = r'''  const ensureCourierProfile = async (token, profile) => {
    if (!token || profile.role !== 'courier') {
      return '';
    }

    const params = new URLSearchParams({ email: profile.email });
    const lookup = await fetch(`/api/v1/couriers/by-email?${params.toString()}`, {
      headers: authHeaders(token),
    });
    if (lookup.ok) {
      const data = await lookup.json();
      return data.courier_id || '';
    }
    if (lookup.status !== 404) {
      throw new Error((await lookup.text()) || `HTTP ${lookup.status}`);
    }

    const create = await fetch('/api/v1/couriers/register', {
      method: 'POST',
      headers: authHeaders(token, true),
      body: JSON.stringify({
        email: profile.email,
        full_name: profile.name,
        phone: profile.phone,
        transport_type: profile.transportType || 'bicycle',
      }),
    });
    if (!create.ok) {
      throw new Error((await create.text()) || `HTTP ${create.status}`);
    }
    const data = await create.json();
    return data.courier_id || '';
  };

'''
write(app, text[:idx] + helper + text[idx:])

replace_once(
    app,
    '''          updateSession({
            token: loginData.token || '',
            profile: {
              ...nextProfile,
              role: loginData.role || nextProfile.role,
              courierId: loginData.courier_id || nextProfile.courierId,
            },
          });
          setCurrentView(nextProfile.role === 'courier' ? 'courier' : 'order');
          setAuthNotice('Регистрация и вход выполнены.');
''',
    '''          const loggedProfile = {
            ...nextProfile,
            name: loginData.name || nextProfile.name,
            phone: loginData.phone || nextProfile.phone,
            email: loginData.email || nextProfile.email,
            role: loginData.role || nextProfile.role,
          };
          loggedProfile.courierId = await ensureCourierProfile(loginData.token || '', loggedProfile);
          updateSession({ token: loginData.token || '', profile: loggedProfile });
          setProfileForm(loggedProfile);
          setCurrentView(loggedProfile.role === 'courier' ? 'courier' : 'order');
          setAuthNotice('Регистрация и вход выполнены.');
''',
)
replace_once(
    app,
    '''        const nextProfile = {
          name: session.profile.name || '',
          phone: session.profile.phone || '',
          email: authForm.login.includes('@') ? authForm.login.trim() : session.profile.email || '',
          role: loginData.role || session.profile.role || 'client',
          courierId: loginData.courier_id || session.profile.courierId || '',
          transportType: session.profile.transportType || 'bicycle',
        };

        updateSession({ token: loginData.token || '', profile: nextProfile });
''',
    '''        const nextProfile = {
          name: loginData.name || session.profile.name || '',
          phone: loginData.phone || session.profile.phone || '',
          email: loginData.email || (authForm.login.includes('@') ? authForm.login.trim() : session.profile.email || ''),
          role: loginData.role || session.profile.role || 'client',
          courierId: '',
          transportType: session.profile.transportType || 'bicycle',
        };
        nextProfile.courierId = await ensureCourierProfile(loginData.token || '', nextProfile);

        updateSession({ token: loginData.token || '', profile: nextProfile });
''',
)

replace_once(app, "const response = await fetch('/api/v1/orders', { signal: controller.signal });", "const response = await fetch('/api/v1/orders', { signal: controller.signal, headers: authHeaders(session.token) });")
replace_once(
    app,
    '''        const data = await response.json();
        const availableOrders = Array.isArray(data)
          ? data
            .map(normalizeBackendOrder)
''',
    '''        const data = await response.json();
        const availableOrders = extractOrders(data)
            .map(normalizeBackendOrder)
''',
)
replace_once(app, '''            .filter((order) => !acceptedCourierOrderIds.includes(order.id))
          : [];
''', '''            .filter((order) => !acceptedCourierOrderIds.includes(order.id));
''')
replace_once(app, "headers: { 'Content-Type': 'application/json' },\n        body: JSON.stringify({\n          courier_id: courierId,\n          is_online:", "headers: authHeaders(session.token, true),\n        body: JSON.stringify({\n          courier_id: courierId,\n          is_online:")
replace_once(app, "headers: { 'Content-Type': 'application/json' },\n        body: JSON.stringify({\n          courier_id: courierId,\n          mode: 'manual',", "headers: authHeaders(session.token, true),\n        body: JSON.stringify({\n          courier_id: courierId,\n          mode: 'manual',")
replace_once(app, "headers: { 'Content-Type': 'application/json' },\n        body: JSON.stringify({\n          courier_id: courierId,\n          status: nextStatus,", "headers: authHeaders(session.token, true),\n        body: JSON.stringify({\n          courier_id: courierId,\n          status: nextStatus,")

replace_once(
    app,
    '''      weight: parseFloat(form.weight) || 0,
      distance_km: formRouteInfo.distanceKm || 0,
      user_id: '00000000-0000-0000-0000-000000000000',
''',
    '''      from_coords: { latitude: fromCoords[0], longitude: fromCoords[1] },
      to_coords: { latitude: toCoords[0], longitude: toCoords[1] },
      weight: parseFloat(form.weight) || 0,
''',
)
replace_once(app, '''      const createdOrder = await response.json();

      const order = {
''', '''      const createdPayload = await response.json();
      const createdOrder = createdPayload.order || createdPayload;

      const order = {
''')
replace_once(app, '''        eta: `${Math.max(1, Math.round(formRouteInfo.durationMin || (12 + Math.random() * 20)))} мин`,
''', '''        eta: `${Math.max(1, Math.round(createdPayload.estimated_duration_minutes || formRouteInfo.durationMin || 1))} мин`,
''')

Path('.github/workflows/runtime-contract-fix.yml').unlink()
Path('scripts/runtime_contract_fix.py').unlink()
