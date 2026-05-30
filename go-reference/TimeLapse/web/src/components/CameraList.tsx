import { useState } from 'react';
import type { Camera } from '../types';
import CameraCard from './CameraCard';

interface CameraListProps {
  cameras: Camera[];
  onCameraDeleted: (uuid: string) => void;
  onRefresh: () => void;
}

export default function CameraList({ cameras, onCameraDeleted, onRefresh }: CameraListProps) {
  const [expandedCamera, setExpandedCamera] = useState<string | null>(null);

  return (
    <div>
      {cameras.map((camera) => (
        <CameraCard
          key={camera.uuid}
          camera={camera}
          isExpanded={expandedCamera === camera.uuid}
          onToggleExpand={() =>
            setExpandedCamera(expandedCamera === camera.uuid ? null : camera.uuid)
          }
          onDeleted={() => onCameraDeleted(camera.uuid)}
          onRefresh={onRefresh}
        />
      ))}
    </div>
  );
}
