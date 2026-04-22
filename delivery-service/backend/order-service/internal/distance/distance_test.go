package distance

import (
	"math"
	"testing"
)

func TestCalculateDistance(t *testing.T) {
	type testCase struct {
		name     string
		lat1     float64
		lon1     float64
		lat2     float64
		lon2     float64
		expected float64
		tol      float64
	}

	tests := []testCase{
		{
			name: "Одинаковые точки (дистанция 0)",
			lat1: 55.7558, lon1: 37.6173,
			lat2: 55.7558, lon2: 37.6173,
			expected: 0.0,
			tol:      0.001,
		},
		{
			name: "Москва -> Санкт-Петербург",
			lat1: 55.7558, lon1: 37.6173,
			lat2: 59.9343, lon2: 30.3351,
			expected: 634.0,
			tol:      2.0,
		},
		{
			name: "Экватор (сдвиг ровно на 1 градус долготы)",
			lat1: 0.0, lon1: 0.0,
			lat2: 0.0, lon2: 1.0,
			expected: 111.19,
			tol:      0.1,
		},
		{
			name: "Отрицательные координаты (Рио-де-Жанейро -> Сидней)",
			lat1: -22.9068, lon1: -43.1729,
			lat2: -33.8688, lon2: 151.2093,
			expected: 13522.0,
			tol:      15.0,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := CalculateDistance(tc.lat1, tc.lon1, tc.lat2, tc.lon2)

			diff := math.Abs(got - tc.expected)
			if diff > tc.tol {
				t.Errorf("CalculateDistance() = %v км, ожидалось %v км (разница %v больше допустимой %v)",
					got, tc.expected, diff, tc.tol)
			}
		})
	}
}
