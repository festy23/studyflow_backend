package data

import (
	"fmt"
	"github.com/google/uuid"
	"strings"
	"userservice/internal/model"
)

var ErrNoFieldsToUpdate = fmt.Errorf("no fields to update")

func buildUserUpdateQuery(input *model.UpdateUserInput) (string, []any, error) {
	var set []string
	var args []any
	argIdx := 1

	if input.FirstName != nil {
		set = append(set, fmt.Sprintf("first_name = $%d", argIdx))
		args = append(args, input.FirstName)
		argIdx++
	}
	if input.LastName != nil {
		set = append(set, fmt.Sprintf("last_name = $%d", argIdx))
		args = append(args, input.LastName)
		argIdx++
	}
	if input.Timezone != nil {
		set = append(set, fmt.Sprintf("timezone = $%d", argIdx))
		args = append(args, input.Timezone)
		argIdx++
	}

	if len(set) == 0 {
		return "", nil, ErrNoFieldsToUpdate
	}

	query := fmt.Sprintf(
		`
UPDATE users 
SET %s 
WHERE id = $%d 
RETURNING
	id, role, auth_provider, status,
	first_name, last_name, timezone,
	created_at, edited_at

`,
		strings.Join(set, ", "),
		argIdx,
	)
	return query, args, nil
}

func buildTutorProfileUpdateQuery(input *model.UpdateTutorProfileInput) (string, []any, error) {
	var set []string
	var args []any
	argIdx := 1

	if input.PaymentInfo != nil {
		set = append(set, fmt.Sprintf("payment_info = $%d", argIdx))
		args = append(args, input.PaymentInfo)
		argIdx++
	}
	if input.LessonPriceRub != nil {
		set = append(set, fmt.Sprintf("lesson_price_rub = $%d", argIdx))
		args = append(args, input.LessonPriceRub)
		argIdx++
	}
	if input.LessonConnectionLink != nil {
		set = append(set, fmt.Sprintf("lesson_connection_link = $%d", argIdx))
		args = append(args, input.LessonConnectionLink)
		argIdx++
	}

	if len(set) == 0 {
		return "", nil, ErrNoFieldsToUpdate
	}

	query := fmt.Sprintf(`
UPDATE tutor_profiles
SET %s
WHERE user_id = $%d
RETURNING id, user_id, payment_info, lesson_price_rub, lesson_connection_link, created_at, edited_at
`,
		strings.Join(set, ", "),
		argIdx,
	)

	return query, args, nil
}

func buildUpdateTutorStudentQuery(input *model.UpdateTutorStudentInput) (string, []any, error) {
	var set []string
	var args []any
	argIdx := 1

	if input.LessonPriceRub != nil {
		set = append(set, fmt.Sprintf("lesson_price_rub = $%d", argIdx))
		args = append(args, input.LessonPriceRub)
		argIdx++
	}
	if input.LessonConnectionLink != nil {
		set = append(set, fmt.Sprintf("lesson_connection_link = $%d", argIdx))
		args = append(args, input.LessonConnectionLink)
		argIdx++
	}
	if input.Status != nil {
		set = append(set, fmt.Sprintf("status = $%d", argIdx))
		args = append(args, input.Status)
		argIdx++
	}

	if len(set) == 0 {
		return "", nil, ErrNoFieldsToUpdate
	}

	query := fmt.Sprintf(`
UPDATE tutor_students
SET %s
WHERE tutor_id = $%d AND student_id = $%d
RETURNING id, tutor_id, student_id,
    lesson_price_rub, lesson_connection_link,
    status, created_at, edited_at
`, strings.Join(set, ", "), argIdx, argIdx+1)

	return query, args, nil
}

func buildListTutorStudentsQuery(tutorID uuid.UUID, studentID uuid.UUID) (string, []any) {
	var where []string
	var args []any
	argIdx := 1

	if tutorID != uuid.Nil {
		where = append(where, fmt.Sprintf("tutor_id = $%d", argIdx))
		args = append(args, tutorID)
		argIdx++
	}
	if studentID != uuid.Nil {
		where = append(where, fmt.Sprintf("student_id = $%d", argIdx))
		args = append(args, studentID)
		argIdx++
	}

	query := `
SELECT 
    id, tutor_id, student_id, lesson_price_rub, 
    lesson_connection_link, status, 
    created_at, edited_at
FROM tutor_students
`
	if len(where) > 0 {
		query += "WHERE " + strings.Join(where, " AND ") + "\n"
	}

	return query, args
}
