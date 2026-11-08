import { useCallback } from "react";

function openFileInput(accept: string, capture?: string): Promise<File | null> {
  return new Promise((resolve) => {
    const input = document.createElement("input");
    input.type = "file";
    input.accept = accept;
    if (capture) input.setAttribute("capture", capture);
    input.style.display = "none";
    document.body.appendChild(input);

    input.onchange = () => {
      const file = input.files?.[0] ?? null;
      document.body.removeChild(input);
      resolve(file);
    };

    // Resolve null if dialog is dismissed without selecting a file
    input.oncancel = () => {
      document.body.removeChild(input);
      resolve(null);
    };

    input.click();
  });
}

export function useCamera() {
  // Opens the device camera (on mobile) or file picker (on desktop)
  const openCamera = useCallback(
    () => openFileInput("image/*", "environment"),
    []
  );

  // Opens a generic image file picker (gallery / filesystem)
  const pickImage = useCallback(
    () => openFileInput("image/*"),
    []
  );

  return { openCamera, pickImage };
}
