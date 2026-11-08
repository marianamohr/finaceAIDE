const MAX_FILE_SIZE = 2 * 1024 * 1024; // 2 MB
const MAX_DIMENSION = 1920;             // px

/**
 * Resizes the image if it exceeds MAX_FILE_SIZE or MAX_DIMENSION on either axis.
 * Always re-encodes as JPEG at 85% quality when resizing.
 * Camera photos (e.g. iPhone 4032×3024) are resized even if under 2 MB.
 */
export async function resizeIfNeeded(file: File): Promise<File> {
  return new Promise((resolve, reject) => {
    const img = new Image();
    const url = URL.createObjectURL(file);

    img.onload = () => {
      URL.revokeObjectURL(url);

      const oversizedFile = file.size > MAX_FILE_SIZE;
      const oversizedDim = img.width > MAX_DIMENSION || img.height > MAX_DIMENSION;

      if (!oversizedFile && !oversizedDim) {
        resolve(file);
        return;
      }

      const scale = Math.min(MAX_DIMENSION / img.width, MAX_DIMENSION / img.height, 1);
      const canvas = document.createElement("canvas");
      canvas.width  = Math.round(img.width  * scale);
      canvas.height = Math.round(img.height * scale);

      const ctx = canvas.getContext("2d");
      if (!ctx) {
        reject(new Error("resizeImage: could not get canvas context"));
        return;
      }
      ctx.drawImage(img, 0, 0, canvas.width, canvas.height);

      canvas.toBlob(
        (blob) => {
          if (!blob) {
            reject(new Error("resizeImage: toBlob returned null"));
            return;
          }
          resolve(new File([blob], file.name.replace(/\.[^.]+$/, ".jpg"), { type: "image/jpeg" }));
        },
        "image/jpeg",
        0.85,
      );
    };

    img.onerror = () => {
      URL.revokeObjectURL(url);
      reject(new Error("resizeImage: failed to load image"));
    };

    img.src = url;
  });
}
