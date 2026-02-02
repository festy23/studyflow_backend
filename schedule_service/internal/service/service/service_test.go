package service_test

import (
	"common_library/ctxdata"
	"context"
	"testing"
	"time"
	userpb "userservice/pkg/api"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"

	"schedule_service/internal/database/repo"
	"schedule_service/internal/service/service"
	pb "schedule_service/pkg/api"
	"schedule_service/pkg/mocks"
)

func setup(t *testing.T) (*service.ScheduleServer, *mocks.MockRepository, *mocks.MockIUserClient, *gomock.Controller) {
	ctrl := gomock.NewController(t)
	t.Cleanup(ctrl.Finish)

	mockRepo := mocks.NewMockRepository(ctrl)
	mockUserClient := mocks.NewMockIUserClient(ctrl)
	srv := service.NewScheduleServer(mockRepo, mockUserClient)

	return srv, mockRepo, mockUserClient, ctrl
}

func TestGetSlot(t *testing.T) {
	t.Run("Success", func(t *testing.T) {
		srv, mockRepo, _, _ := setup(t)
		ctx := ctxdata.WithUserID(context.Background(), "de305d54-75b4-431b-adb2-eb6b9e546014")

		slotID := "5d2d6f89-5bfa-4d85-bd3e-fcb4d6f56f20"
		now := time.Now()

		mockRepo.EXPECT().GetSlot(ctx, slotID).Return(&repo.Slot{
			ID:        slotID,
			TutorID:   "de305d54-75b4-431b-adb2-eb6b9e546014",
			StartsAt:  now,
			EndsAt:    now.Add(time.Hour),
			IsBooked:  false,
			CreatedAt: now,
			EditedAt:  &now,
		}, nil)

		resp, err := srv.GetSlot(ctx, &pb.GetSlotRequest{Id: slotID})
		require.NoError(t, err)
		require.Equal(t, slotID, resp.Id)
		require.Equal(t, "de305d54-75b4-431b-adb2-eb6b9e546014", resp.TutorId)
		require.False(t, resp.IsBooked)
		require.WithinDuration(t, now, resp.StartsAt.AsTime(), time.Second)
	})

	t.Run("Unauthenticated", func(t *testing.T) {
		srv, _, _, _ := setup(t)
		ctx := context.Background()

		_, err := srv.GetSlot(ctx, &pb.GetSlotRequest{Id: "abc"})
		require.Error(t, err)
		st, _ := status.FromError(err)
		require.Equal(t, codes.Unauthenticated, st.Code())
	})

	t.Run("Invalid UUID", func(t *testing.T) {
		srv, _, _, _ := setup(t)
		ctx := ctxdata.WithUserID(context.Background(), "1")

		_, err := srv.GetSlot(ctx, &pb.GetSlotRequest{Id: "not-a-uuid"})
		require.Error(t, err)
		st, _ := status.FromError(err)
		require.Equal(t, codes.InvalidArgument, st.Code())
	})

	t.Run("Slot Not Found", func(t *testing.T) {
		srv, mockRepo, _, _ := setup(t)
		ctx := ctxdata.WithUserID(context.Background(), "de305d54-75b4-431b-adb2-eb6b9e546014")

		slotID := "de305d54-75b4-431b-adb2-eb6b9e546014"
		mockRepo.EXPECT().GetSlot(ctx, slotID).Return(nil, service.ErrSlotNotFound)

		_, err := srv.GetSlot(ctx, &pb.GetSlotRequest{Id: slotID})
		require.Error(t, err)
		st, _ := status.FromError(err)
		require.Equal(t, codes.NotFound, st.Code())
	})
}
func TestCreateSlot(t *testing.T) {

	t.Run("Success", func(t *testing.T) {
		srv, mockRepo, mockUserClient, _ := setup(t)
		tutorID := "de305d54-75b4-431b-adb2-eb6b9e546014"
		ctx := ctxdata.WithUserID(context.Background(), tutorID)
		ctx = ctxdata.WithUserRole(ctx, "tutor")

		startTime := time.Now().Add(time.Hour)
		endTime := startTime.Add(time.Hour)

		mockUserClient.EXPECT().GetTutorStudent(gomock.Any(), tutorID, tutorID).Return(&userpb.TutorStudent{Status: "active"}, nil).AnyTimes()
		mockRepo.EXPECT().CreateSlot(gomock.Any(), gomock.Any()).DoAndReturn(
			func(_ context.Context, slot repo.Slot) error {
				require.Equal(t, tutorID, slot.TutorID)
				require.WithinDuration(t, startTime, slot.StartsAt, time.Second)
				require.WithinDuration(t, endTime, slot.EndsAt, time.Second)
				require.False(t, slot.IsBooked)
				return nil
			},
		)

		resp, err := srv.CreateSlot(ctx, &pb.CreateSlotRequest{
			TutorId:  tutorID,
			StartsAt: timestamppb.New(startTime),
			EndsAt:   timestamppb.New(endTime),
		})

		require.NoError(t, err)
		require.Equal(t, tutorID, resp.TutorId)
		require.WithinDuration(t, startTime, resp.StartsAt.AsTime(), time.Second)
		require.WithinDuration(t, endTime, resp.EndsAt.AsTime(), time.Second)
		require.False(t, resp.IsBooked)
	})

	t.Run("Unauthenticated", func(t *testing.T) {
		srv, _, _, _ := setup(t)
		ctx := context.Background()

		_, err := srv.CreateSlot(ctx, &pb.CreateSlotRequest{})
		require.Error(t, err)
		st, _ := status.FromError(err)
		require.Equal(t, codes.Unauthenticated, st.Code())
	})

	t.Run("Not a Tutor", func(t *testing.T) {
		srv, _, _, _ := setup(t)
		userID := "de305d54-75b4-431b-adb2-eb6b9e546014"
		ctx := ctxdata.WithUserID(context.Background(), userID)
		ctx = ctxdata.WithUserRole(ctx, "student")

		_, err := srv.CreateSlot(ctx, &pb.CreateSlotRequest{
			TutorId: userID,
		})
		require.Error(t, err)
		st, _ := status.FromError(err)
		require.Equal(t, codes.PermissionDenied, st.Code())
	})

	t.Run("Invalid Time Range", func(t *testing.T) {
		srv, _, mockUserClient, _ := setup(t)
		tutorID := "de305d54-75b4-431b-adb2-eb6b9e546014"
		ctx := ctxdata.WithUserID(context.Background(), tutorID)
		ctx = ctxdata.WithUserRole(ctx, "tutor")

		startTime := time.Now().Add(time.Hour)
		endTime := startTime.Add(-time.Minute)

		mockUserClient.EXPECT().GetTutorStudent(gomock.Any(), tutorID, tutorID).Return(&userpb.TutorStudent{Status: "active"}, nil).AnyTimes()

		_, err := srv.CreateSlot(ctx, &pb.CreateSlotRequest{
			TutorId:  tutorID,
			StartsAt: timestamppb.New(startTime),
			EndsAt:   timestamppb.New(endTime),
		})
		require.Error(t, err)
		st, _ := status.FromError(err)
		require.Equal(t, codes.InvalidArgument, st.Code())
	})
}

func TestUpdateSlot(t *testing.T) {
	t.Run("Success", func(t *testing.T) {
		srv, mockRepo, _, _ := setup(t)
		tutorID := "de305d54-75b4-431b-adb2-eb6b9e546014"
		slotID := "5d2d6f89-5bfa-4d85-bd3e-fcb4d6f56f20"
		ctx := ctxdata.WithUserID(context.Background(), tutorID)

		now := time.Now()
		startTime := now.Add(time.Hour)
		endTime := startTime.Add(time.Hour)

		existingSlot := &repo.Slot{
			ID:        slotID,
			TutorID:   tutorID,
			StartsAt:  now,
			EndsAt:    now.Add(30 * time.Minute),
			IsBooked:  false,
			CreatedAt: now.Add(-time.Hour),
		}

		mockRepo.EXPECT().GetSlot(gomock.Any(), slotID).Return(existingSlot, nil)
		mockRepo.EXPECT().UpdateSlot(gomock.Any(), gomock.Any()).DoAndReturn(
			func(_ context.Context, slot repo.Slot) error {
				require.Equal(t, slotID, slot.ID)
				require.Equal(t, tutorID, slot.TutorID)
				require.WithinDuration(t, startTime, slot.StartsAt, time.Second)
				require.WithinDuration(t, endTime, slot.EndsAt, time.Second)
				require.False(t, slot.IsBooked)
				require.NotNil(t, slot.EditedAt)
				return nil
			},
		)

		resp, err := srv.UpdateSlot(ctx, &pb.UpdateSlotRequest{
			Id:       slotID,
			StartsAt: timestamppb.New(startTime),
			EndsAt:   timestamppb.New(endTime),
		})

		require.NoError(t, err)
		require.Equal(t, slotID, resp.Id)
		require.Equal(t, tutorID, resp.TutorId)
		require.WithinDuration(t, startTime, resp.StartsAt.AsTime(), time.Second)
		require.WithinDuration(t, endTime, resp.EndsAt.AsTime(), time.Second)
		require.False(t, resp.IsBooked)
		require.NotNil(t, resp.EditedAt)
	})

	t.Run("Slot Not Found", func(t *testing.T) {
		srv, mockRepo, _, _ := setup(t)
		userID := "fe305d54-75b4-431b-adb2-eb6b9e546"
		slotID := "de305d54-75b4-431b-adb2-eb6b9e546014"
		ctx := ctxdata.WithUserID(context.Background(), userID)

		mockRepo.EXPECT().GetSlot(gomock.Any(), slotID).Return(nil, service.ErrSlotNotFound)

		_, err := srv.UpdateSlot(ctx, &pb.UpdateSlotRequest{Id: slotID})
		require.Error(t, err)
		st, _ := status.FromError(err)
		require.Equal(t, codes.NotFound, st.Code())
	})

	t.Run("Booked Slot", func(t *testing.T) {
		srv, mockRepo, _, _ := setup(t)
		tutorID := "de305d54-75b4-431b-adb2-eb6b9e546014"
		slotID := "5d2d6f89-5bfa-4d85-bd3e-fcb4d6f56f20"
		ctx := ctxdata.WithUserID(context.Background(), tutorID)

		now := time.Now()
		existingSlot := &repo.Slot{
			ID:        slotID,
			TutorID:   tutorID,
			StartsAt:  now.Add(time.Hour),
			EndsAt:    now.Add(2 * time.Hour),
			IsBooked:  true,
			CreatedAt: now.Add(-time.Hour),
		}

		mockRepo.EXPECT().GetSlot(gomock.Any(), slotID).Return(existingSlot, nil)

		_, err := srv.UpdateSlot(ctx, &pb.UpdateSlotRequest{
			Id:       slotID,
			StartsAt: timestamppb.New(now.Add(3 * time.Hour)),
			EndsAt:   timestamppb.New(now.Add(4 * time.Hour)),
		})
		require.Error(t, err)
		st, _ := status.FromError(err)
		require.Equal(t, codes.FailedPrecondition, st.Code())
	})
}

func TestDeleteSlot(t *testing.T) {
	t.Run("Success", func(t *testing.T) {
		srv, mockRepo, _, _ := setup(t)
		tutorID := "de305d54-75b4-431b-adb2-eb6b9e546014"
		slotID := "5d2d6f89-5bfa-4d85-bd3e-fcb4d6f56f20"
		ctx := ctxdata.WithUserID(context.Background(), tutorID)

		now := time.Now()
		existingSlot := &repo.Slot{
			ID:        slotID,
			TutorID:   tutorID,
			StartsAt:  now.Add(time.Hour),
			EndsAt:    now.Add(2 * time.Hour),
			IsBooked:  false,
			CreatedAt: now.Add(-time.Hour),
		}

		mockRepo.EXPECT().GetSlot(gomock.Any(), slotID).Return(existingSlot, nil)
		mockRepo.EXPECT().DeleteSlot(gomock.Any(), slotID).Return(nil)

		resp, err := srv.DeleteSlot(ctx, &pb.DeleteSlotRequest{Id: slotID})
		require.NoError(t, err)
		require.NotNil(t, resp)
	})

	t.Run("Not Owner", func(t *testing.T) {
		srv, mockRepo, _, _ := setup(t)
		userID := "de305d54-75b4-431b-adb2-eb6b9e546014"
		tutorID := "de305d54-75b4-431b-adb2-eb6b9e546015"
		slotID := "5d2d6f89-5bfa-4d85-bd3e-fcb4d6f56f20"
		ctx := ctxdata.WithUserID(context.Background(), userID)

		now := time.Now()
		existingSlot := &repo.Slot{
			ID:        slotID,
			TutorID:   tutorID,
			StartsAt:  now.Add(time.Hour),
			EndsAt:    now.Add(2 * time.Hour),
			IsBooked:  false,
			CreatedAt: now.Add(-time.Hour),
		}

		mockRepo.EXPECT().GetSlot(gomock.Any(), slotID).Return(existingSlot, nil)

		_, err := srv.DeleteSlot(ctx, &pb.DeleteSlotRequest{Id: slotID})
		require.Error(t, err)
		st, _ := status.FromError(err)
		require.Equal(t, codes.PermissionDenied, st.Code())
	})
}

func TestListSlotsByTutor(t *testing.T) {
	t.Run("Success - Own Slots", func(t *testing.T) {
		srv, mockRepo, _, _ := setup(t)
		tutorID := "de305d54-75b4-431b-adb2-eb6b9e546014"
		ctx := ctxdata.WithUserID(context.Background(), tutorID)

		now := time.Now()
		slots := []repo.Slot{
			{
				ID:        "de305d54-75b4-431b-adb2-eb6b9e546017",
				TutorID:   tutorID,
				StartsAt:  now.Add(time.Hour),
				EndsAt:    now.Add(2 * time.Hour),
				IsBooked:  false,
				CreatedAt: now.Add(-time.Hour),
			},
			{
				ID:        "slot-2",
				TutorID:   tutorID,
				StartsAt:  now.Add(3 * time.Hour),
				EndsAt:    now.Add(4 * time.Hour),
				IsBooked:  true,
				CreatedAt: now.Add(-2 * time.Hour),
			},
		}

		onlyAvailable := false
		mockRepo.EXPECT().ListSlotsByTutor(gomock.Any(), tutorID, onlyAvailable).Return(slots, nil)

		resp, err := srv.ListSlotsByTutor(ctx, &pb.ListSlotsByTutorRequest{
			TutorId: tutorID,
		})
		require.NoError(t, err)
		require.Len(t, resp.Slots, 2)
		require.Equal(t, "de305d54-75b4-431b-adb2-eb6b9e546017", resp.Slots[0].Id)
		require.Equal(t, "slot-2", resp.Slots[1].Id)
		require.False(t, resp.Slots[0].IsBooked)
		require.True(t, resp.Slots[1].IsBooked)
	})

	t.Run("Success - Only Available", func(t *testing.T) {
		srv, mockRepo, _, _ := setup(t)
		tutorID := "de305d54-75b4-431b-adb2-eb6b9e546014"
		ctx := ctxdata.WithUserID(context.Background(), tutorID)

		now := time.Now()
		slots := []repo.Slot{
			{
				ID:        "de305d54-75b4-431b-adb2-eb6b9e546017",
				TutorID:   tutorID,
				StartsAt:  now.Add(time.Hour),
				EndsAt:    now.Add(2 * time.Hour),
				IsBooked:  false,
				CreatedAt: now.Add(-time.Hour),
			},
		}

		onlyAvailable := true
		onlyAvailablePointer := true
		mockRepo.EXPECT().ListSlotsByTutor(gomock.Any(), tutorID, onlyAvailable).Return(slots, nil)

		resp, err := srv.ListSlotsByTutor(ctx, &pb.ListSlotsByTutorRequest{
			TutorId:       tutorID,
			OnlyAvailable: &onlyAvailablePointer,
		})
		require.NoError(t, err)
		require.Len(t, resp.Slots, 1)
		require.Equal(t, "de305d54-75b4-431b-adb2-eb6b9e546017", resp.Slots[0].Id)
		require.False(t, resp.Slots[0].IsBooked)
	})
}

func TestGetLesson(t *testing.T) {
	t.Run("Success - Tutor", func(t *testing.T) {
		srv, mockRepo, _, _ := setup(t)
		tutorID := "de305d54-75b4-431b-adb2-eb6b9e546014"
		studentID := "de305d54-75b4-431b-adb2-eb6b9e546015"
		lessonID := "de305d54-75b4-431b-adb2-eb6b9e546015"
		slotID := "de305d54-75b4-431b-adb2-eb6b9e546017"
		ctx := ctxdata.WithUserID(context.Background(), tutorID)

		now := time.Now()
		lesson := &repo.Lesson{
			ID:        lessonID,
			SlotID:    slotID,
			StudentID: studentID,
			Status:    "booked",
			IsPaid:    false,
			CreatedAt: now.Add(-time.Hour),
			EditedAt:  now,
		}

		slot := &repo.Slot{
			ID:        slotID,
			TutorID:   tutorID,
			StartsAt:  now.Add(time.Hour),
			EndsAt:    now.Add(2 * time.Hour),
			IsBooked:  true,
			CreatedAt: now.Add(-time.Hour),
		}

		mockRepo.EXPECT().GetLesson(gomock.Any(), lessonID).Return(lesson, nil)
		mockRepo.EXPECT().GetSlot(gomock.Any(), slotID).Return(slot, nil)

		resp, err := srv.GetLesson(ctx, &pb.GetLessonRequest{Id: lessonID})
		require.NoError(t, err)
		require.Equal(t, lessonID, resp.Id)
		require.Equal(t, slotID, resp.SlotId)
		require.Equal(t, studentID, resp.StudentId)
		require.Equal(t, "booked", resp.Status)
		require.False(t, resp.IsPaid)
	})

	t.Run("Success - Student", func(t *testing.T) {
		srv, mockRepo, _, _ := setup(t)
		tutorID := "de305d54-75b4-431b-adb2-eb6b9e546014"
		studentID := "de305d54-75b4-431b-adb2-eb6b9e546015"
		lessonID := "de305d54-75b4-431b-adb2-eb6b9e546016"
		slotID := "de305d54-75b4-431b-adb2-eb6b9e546017"
		ctx := ctxdata.WithUserID(context.Background(), studentID)

		now := time.Now()
		lesson := &repo.Lesson{
			ID:        lessonID,
			SlotID:    slotID,
			StudentID: studentID,
			Status:    "booked",
			IsPaid:    false,
			CreatedAt: now.Add(-time.Hour),
			EditedAt:  now,
		}

		slot := &repo.Slot{
			ID:        slotID,
			TutorID:   tutorID,
			StartsAt:  now.Add(time.Hour),
			EndsAt:    now.Add(2 * time.Hour),
			IsBooked:  true,
			CreatedAt: now.Add(-time.Hour),
		}

		mockRepo.EXPECT().GetLesson(gomock.Any(), lessonID).Return(lesson, nil)
		mockRepo.EXPECT().GetSlot(gomock.Any(), slotID).Return(slot, nil)

		resp, err := srv.GetLesson(ctx, &pb.GetLessonRequest{Id: lessonID})
		require.NoError(t, err)
		require.Equal(t, lessonID, resp.Id)
		require.Equal(t, slotID, resp.SlotId)
		require.Equal(t, studentID, resp.StudentId)
		require.Equal(t, "booked", resp.Status)
		require.False(t, resp.IsPaid)
	})

	t.Run("Lesson Not Found", func(t *testing.T) {
		srv, mockRepo, _, _ := setup(t)
		userID := "de305d54-75b4-431b-adb2-eb6b9e546014"
		lessonID := "de305d54-75b4-431b-adb2-eb6b9e546015"
		ctx := ctxdata.WithUserID(context.Background(), userID)

		mockRepo.EXPECT().GetLesson(gomock.Any(), lessonID).Return(nil, service.ErrLessonNotFound)

		_, err := srv.GetLesson(ctx, &pb.GetLessonRequest{Id: lessonID})
		require.Error(t, err)
		st, _ := status.FromError(err)
		require.Equal(t, codes.NotFound, st.Code())
	})

}

func TestCreateLesson(t *testing.T) {
	t.Run("Success", func(t *testing.T) {
		srv, mockRepo, mockUserClient, _ := setup(t)
		tutorID := "de305d54-75b4-431b-adb2-eb6b9e546014"
		studentID := "de305d54-75b4-431b-adb2-eb6b9e546015"
		slotID := "de305d54-75b4-431b-adb2-eb6b9e546016"
		ctx := ctxdata.WithUserID(context.Background(), studentID)
		ctx = ctxdata.WithUserRole(ctx, "student")

		now := time.Now()
		slot := &repo.Slot{
			ID:        slotID,
			TutorID:   tutorID,
			StartsAt:  now.Add(time.Hour),
			EndsAt:    now.Add(2 * time.Hour),
			IsBooked:  false,
			CreatedAt: now.Add(-time.Hour),
		}

		mockRepo.EXPECT().GetSlot(gomock.Any(), slotID).Return(slot, nil)
		mockUserClient.EXPECT().GetTutorStudent(gomock.Any(), tutorID, studentID).Return(&userpb.TutorStudent{Status: "active"}, nil)
		mockRepo.EXPECT().CreateLessonAndBookSlot(gomock.Any(), gomock.Any(), slotID).Return(nil)

		resp, err := srv.CreateLesson(ctx, &pb.CreateLessonRequest{
			SlotId:    slotID,
			StudentId: studentID,
		})
		require.NoError(t, err)
		require.Equal(t, slotID, resp.SlotId)
		require.Equal(t, studentID, resp.StudentId)
		require.Equal(t, "booked", resp.Status)
		require.False(t, resp.IsPaid)
	})

	t.Run("Already Booked", func(t *testing.T) {
		srv, mockRepo, _, _ := setup(t)
		tutorID := "de305d54-75b4-431b-adb2-eb6b9e546014"
		studentID := "de305d54-75b4-431b-adb2-eb6b9e546015"
		slotID := "de305d54-75b4-431b-adb2-eb6b9e546016"
		ctx := ctxdata.WithUserID(context.Background(), studentID)

		now := time.Now()
		slot := &repo.Slot{
			ID:        slotID,
			TutorID:   tutorID,
			StartsAt:  now.Add(time.Hour),
			EndsAt:    now.Add(2 * time.Hour),
			IsBooked:  true,
			CreatedAt: now.Add(-time.Hour),
		}

		mockRepo.EXPECT().GetSlot(gomock.Any(), slotID).Return(slot, nil)

		_, err := srv.CreateLesson(ctx, &pb.CreateLessonRequest{
			SlotId:    slotID,
			StudentId: studentID,
		})
		require.Error(t, err)
		st, _ := status.FromError(err)
		require.Equal(t, codes.AlreadyExists, st.Code())
	})
}

func TestUpdateLesson(t *testing.T) {
	t.Run("Success", func(t *testing.T) {
		srv, mockRepo, _, _ := setup(t)
		tutorID := "de305d54-75b4-431b-adb2-eb6b9e546014"
		studentID := "de305d54-75b4-431b-adb2-eb6b9e546015"
		lessonID := "de305d54-75b4-431b-adb2-eb6b9e546016"
		slotID := "de305d54-75b4-431b-adb2-eb6b9e546017"
		ctx := ctxdata.WithUserID(context.Background(), tutorID)

		now := time.Now()
		lesson := &repo.Lesson{
			ID:        lessonID,
			SlotID:    slotID,
			StudentID: studentID,
			Status:    "booked",
			IsPaid:    false,
			CreatedAt: now.Add(-time.Hour),
			EditedAt:  now,
		}

		slot := &repo.Slot{
			ID:        slotID,
			TutorID:   tutorID,
			StartsAt:  now.Add(time.Hour),
			EndsAt:    now.Add(2 * time.Hour),
			IsBooked:  true,
			CreatedAt: now.Add(-time.Hour),
		}

		connectionLink := "https://meet.example.com/de305d54-75b4-431b-adb2-eb6b9e546014"
		priceRub := int32(1500)
		paymentInfo := "Bank transfer"

		mockRepo.EXPECT().GetLesson(gomock.Any(), lessonID).Return(lesson, nil)
		mockRepo.EXPECT().GetSlot(gomock.Any(), slotID).Return(slot, nil)
		mockRepo.EXPECT().UpdateLesson(gomock.Any(), gomock.Any()).DoAndReturn(
			func(_ context.Context, updatedLesson repo.Lesson) error {
				require.Equal(t, lessonID, updatedLesson.ID)
				require.Equal(t, connectionLink, *updatedLesson.ConnectionLink)
				require.Equal(t, priceRub, *updatedLesson.PriceRub)
				require.Equal(t, paymentInfo, *updatedLesson.PaymentInfo)
				return nil
			},
		)

		resp, err := srv.UpdateLesson(ctx, &pb.UpdateLessonRequest{
			Id:             lessonID,
			ConnectionLink: &connectionLink,
			PriceRub:       &priceRub,
			PaymentInfo:    &paymentInfo,
		})
		require.NoError(t, err)
		require.Equal(t, lessonID, resp.Id)
		require.Equal(t, connectionLink, *resp.ConnectionLink)
		require.Equal(t, priceRub, *resp.PriceRub)
		require.Equal(t, paymentInfo, *resp.PaymentInfo)
	})

	t.Run("Only Tutor Can Update", func(t *testing.T) {
		srv, mockRepo, _, _ := setup(t)
		tutorID := "de305d54-75b4-431b-adb2-eb6b9e546014"
		studentID := "de305d54-75b4-431b-adb2-eb6b9e546015"
		lessonID := "de305d54-75b4-431b-adb2-eb6b9e546016"
		slotID := "de305d54-75b4-431b-adb2-eb6b9e546017"
		ctx := ctxdata.WithUserID(context.Background(), studentID)

		now := time.Now()
		lesson := &repo.Lesson{
			ID:        lessonID,
			SlotID:    slotID,
			StudentID: studentID,
			Status:    "booked",
			IsPaid:    false,
			CreatedAt: now.Add(-time.Hour),
			EditedAt:  now,
		}

		slot := &repo.Slot{
			ID:        slotID,
			TutorID:   tutorID,
			StartsAt:  now.Add(time.Hour),
			EndsAt:    now.Add(2 * time.Hour),
			IsBooked:  true,
			CreatedAt: now.Add(-time.Hour),
		}

		mockRepo.EXPECT().GetLesson(gomock.Any(), lessonID).Return(lesson, nil)
		mockRepo.EXPECT().GetSlot(gomock.Any(), slotID).Return(slot, nil)

		_, err := srv.UpdateLesson(ctx, &pb.UpdateLessonRequest{
			Id:             lessonID,
			ConnectionLink: nil,
		})
		require.Error(t, err)
		st, _ := status.FromError(err)
		require.Equal(t, st.Code(), st.Code())
	})
}

func TestCancelLesson(t *testing.T) {
	t.Run("Success - By Tutor", func(t *testing.T) {
		srv, mockRepo, _, _ := setup(t)
		tutorID := "de305d54-75b4-431b-adb2-eb6b9e546014"
		studentID := "de305d54-75b4-431b-adb2-eb6b9e546015"
		lessonID := "de305d54-75b4-431b-adb2-eb6b9e546016"
		slotID := "de305d54-75b4-431b-adb2-eb6b9e546017"
		ctx := ctxdata.WithUserID(context.Background(), tutorID)

		now := time.Now()
		lesson := &repo.Lesson{
			ID:        lessonID,
			SlotID:    slotID,
			StudentID: studentID,
			Status:    "booked",
			IsPaid:    false,
			CreatedAt: now.Add(-time.Hour),
			EditedAt:  now,
		}

		slot := &repo.Slot{
			ID:        slotID,
			TutorID:   tutorID,
			StartsAt:  now.Add(time.Hour),
			EndsAt:    now.Add(2 * time.Hour),
			IsBooked:  true,
			CreatedAt: now.Add(-time.Hour),
		}

		mockRepo.EXPECT().GetLesson(gomock.Any(), lessonID).Return(lesson, nil)
		mockRepo.EXPECT().GetSlot(gomock.Any(), slotID).Return(slot, nil)
		mockRepo.EXPECT().CancelLessonAndFreeSlot(gomock.Any(), gomock.Any(), slotID).DoAndReturn(
			func(_ context.Context, cancelledLesson repo.Lesson, slotID string) error {
				require.Equal(t, lessonID, cancelledLesson.ID)
				require.Equal(t, "cancelled", cancelledLesson.Status)
				return nil
			},
		)

		_, err := srv.CancelLesson(ctx, &pb.CancelLessonRequest{Id: lesson.ID})
		assert.NoError(t, err)

	})
}
