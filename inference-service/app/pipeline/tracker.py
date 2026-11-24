"""ByteTrack-style multi-object tracker for persistent object identity across video frames."""

from enum import Enum
from typing import Dict, List, Optional, Tuple
import numpy as np

from app.models.base import RawDetection, clamp01


class TrackState(str, Enum):
    TENTATIVE = "tentative"
    CONFIRMED = "confirmed"
    LOST = "lost"
    DELETED = "deleted"


def compute_iou_matrix(boxes_a: np.ndarray, boxes_b: np.ndarray) -> np.ndarray:
    """Computes IoU cost matrix between two sets of [x, y, w, h] boxes.

    Returns matrix of shape (len(boxes_a), len(boxes_b)).
    """
    if len(boxes_a) == 0 or len(boxes_b) == 0:
        return np.zeros((len(boxes_a), len(boxes_b)), dtype=np.float32)

    # Convert [x, y, w, h] to [x1, y1, x2, y2]
    a_x1 = boxes_a[:, 0]
    a_y1 = boxes_a[:, 1]
    a_x2 = boxes_a[:, 0] + boxes_a[:, 2]
    a_y2 = boxes_a[:, 1] + boxes_a[:, 3]

    b_x1 = boxes_b[:, 0]
    b_y1 = boxes_b[:, 1]
    b_x2 = boxes_b[:, 0] + boxes_b[:, 2]
    b_y2 = boxes_b[:, 1] + boxes_b[:, 3]

    area_a = np.maximum(0.0, a_x2 - a_x1) * np.maximum(0.0, a_y2 - a_y1)
    area_b = np.maximum(0.0, b_x2 - b_x1) * np.maximum(0.0, b_y2 - b_y1)

    ious = np.zeros((len(boxes_a), len(boxes_b)), dtype=np.float32)
    for i in range(len(boxes_a)):
        xx1 = np.maximum(a_x1[i], b_x1)
        yy1 = np.maximum(a_y1[i], b_y1)
        xx2 = np.minimum(a_x2[i], b_x2)
        yy2 = np.minimum(a_y2[i], b_y2)

        w = np.maximum(0.0, xx2 - xx1)
        h = np.maximum(0.0, yy2 - yy1)
        inter = w * h
        union = area_a[i] + area_b - inter
        ious[i] = np.where(union > 0, inter / union, 0.0)

    return ious


class STrack:
    """A single persistent track."""

    _count = 0

    def __init__(
        self,
        bbox: Tuple[float, float, float, float],
        score: float,
        class_name: str,
        modality: str = "rgb",
    ):
        STrack._count += 1
        self.track_id = STrack._count
        self.bbox = np.array(bbox, dtype=np.float32)  # [x, y, w, h]
        self.score = float(score)
        self.class_name = class_name
        self.modality = modality

        self.state = TrackState.TENTATIVE
        self.age = 1
        self.hits = 1
        self.time_since_update = 0

    @classmethod
    def reset_counter(cls, start: int = 0):
        cls._count = start

    def update(self, new_bbox: Tuple[float, float, float, float], score: float, alpha: float = 0.75):
        """Updates track box using Exponential Moving Average (EMA) smoothing."""
        meas = np.array(new_bbox, dtype=np.float32)
        self.bbox = alpha * meas + (1.0 - alpha) * self.bbox
        self.score = float(score)
        self.hits += 1
        self.time_since_update = 0

        if self.state == TrackState.TENTATIVE and self.hits >= 2:
            self.state = TrackState.CONFIRMED
        elif self.state == TrackState.LOST:
            self.state = TrackState.CONFIRMED

    def mark_missed(self):
        self.time_since_update += 1
        if self.state == TrackState.CONFIRMED and self.time_since_update > 0:
            self.state = TrackState.LOST


class ByteTracker:
    """ByteTrack association tracker per sensor modality with stable track_ids."""

    def __init__(
        self,
        high_thresh: float = 0.5,
        low_thresh: float = 0.2,
        match_thresh: float = 0.3,
        max_age: int = 15,
        min_hits: int = 2,
    ):
        self.high_thresh = high_thresh
        self.low_thresh = low_thresh
        self.match_thresh = match_thresh  # minimum IoU for association
        self.max_age = max_age
        self.min_hits = min_hits
        self.tracked_stracks: List[STrack] = []
        self.frame_id = 0

    def reset(self):
        self.tracked_stracks = []
        self.frame_id = 0
        STrack.reset_counter()

    def update(
        self,
        detections: List[RawDetection],
        class_names: List[str],
        modality: str = "rgb",
    ) -> List[STrack]:
        """Runs two-stage ByteTrack matching and returns confirmed active tracks."""
        self.frame_id += 1

        # 1. Split detections into high-score and low-score pools
        high_dets: List[Tuple[Tuple[float, float, float, float], float, str]] = []
        low_dets: List[Tuple[Tuple[float, float, float, float], float, str]] = []

        for d in detections:
            cls_name = class_names[d.class_id] if d.class_id < len(class_names) else "unknown"
            bbox = (clamp01(d.x), clamp01(d.y), clamp01(d.w), clamp01(d.h))
            if d.confidence >= self.high_thresh:
                high_dets.append((bbox, d.confidence, cls_name))
            elif d.confidence >= self.low_thresh:
                low_dets.append((bbox, d.confidence, cls_name))

        # 2. Match active tracks with high-score detections
        active_tracks = [t for t in self.tracked_stracks if t.state != TrackState.DELETED]
        unmatched_tracks = list(active_tracks)
        unmatched_high_dets = list(high_dets)

        if active_tracks and high_dets:
            track_boxes = np.array([t.bbox for t in active_tracks])
            det_boxes = np.array([d[0] for d in high_dets])
            iou_mat = compute_iou_matrix(track_boxes, det_boxes)

            matched_tracks_idx = set()
            matched_dets_idx = set()

            # Greedy assignment by max IoU
            while True:
                max_iou = 0.0
                best_t, best_d = -1, -1
                for t_idx in range(len(active_tracks)):
                    if t_idx in matched_tracks_idx:
                        continue
                    for d_idx in range(len(high_dets)):
                        if d_idx in matched_dets_idx:
                            continue
                        # Class consistency check
                        if active_tracks[t_idx].class_name != high_dets[d_idx][2]:
                            continue
                        if iou_mat[t_idx, d_idx] > max_iou:
                            max_iou = iou_mat[t_idx, d_idx]
                            best_t, best_d = t_idx, d_idx

                if max_iou < self.match_thresh or best_t < 0:
                    break

                matched_tracks_idx.add(best_t)
                matched_dets_idx.add(best_d)
                active_tracks[best_t].update(high_dets[best_d][0], high_dets[best_d][1])

            unmatched_tracks = [t for idx, t in enumerate(active_tracks) if idx not in matched_tracks_idx]
            unmatched_high_dets = [d for idx, d in enumerate(high_dets) if idx not in matched_dets_idx]

        # 3. Match remaining tracks with low-score detections (second stage)
        remaining_unmatched_tracks = list(unmatched_tracks)
        if unmatched_tracks and low_dets:
            track_boxes = np.array([t.bbox for t in unmatched_tracks])
            det_boxes = np.array([d[0] for d in low_dets])
            iou_mat = compute_iou_matrix(track_boxes, det_boxes)

            matched_tracks_idx = set()
            matched_low_idx = set()

            while True:
                max_iou = 0.0
                best_t, best_d = -1, -1
                for t_idx in range(len(unmatched_tracks)):
                    if t_idx in matched_tracks_idx:
                        continue
                    for d_idx in range(len(low_dets)):
                        if d_idx in matched_low_idx:
                            continue
                        if unmatched_tracks[t_idx].class_name != low_dets[d_idx][2]:
                            continue
                        if iou_mat[t_idx, d_idx] > max_iou:
                            max_iou = iou_mat[t_idx, d_idx]
                            best_t, best_d = t_idx, d_idx

                if max_iou < self.match_thresh or best_t < 0:
                    break

                matched_tracks_idx.add(best_t)
                matched_low_idx.add(best_d)
                unmatched_tracks[best_t].update(low_dets[best_d][0], low_dets[best_d][1])

            remaining_unmatched_tracks = [
                t for idx, t in enumerate(unmatched_tracks) if idx not in matched_tracks_idx
            ]

        # 4. Handle unmatched tracks
        for t in remaining_unmatched_tracks:
            t.mark_missed()
            if t.state == TrackState.TENTATIVE and t.time_since_update > 1:
                t.state = TrackState.DELETED
            elif t.time_since_update > self.max_age:
                t.state = TrackState.DELETED

        # 5. Start new tentative tracks from unmatched high-score detections
        for bbox, score, cls_name in unmatched_high_dets:
            new_track = STrack(bbox=bbox, score=score, class_name=cls_name, modality=modality)
            # If initial score is high enough (e.g. >= 0.70), confirm immediately
            if score >= 0.70 or self.min_hits <= 1:
                new_track.state = TrackState.CONFIRMED
            self.tracked_stracks.append(new_track)

        # Retain non-deleted tracks
        self.tracked_stracks = [t for t in self.tracked_stracks if t.state != TrackState.DELETED]

        # Return confirmed tracks that were updated or active in this frame
        return [
            t
            for t in self.tracked_stracks
            if t.state == TrackState.CONFIRMED and t.time_since_update == 0
        ]
