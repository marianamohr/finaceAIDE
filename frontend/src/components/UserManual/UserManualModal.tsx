import { Dialog, DialogContent, DialogTitle } from "@/components/ui/dialog";
import { UserManual } from "./UserManual";

interface UserManualModalProps {
  open: boolean;
  context?: "onboarding" | "profile";
  onClose: () => void;
  onViewFull?: () => void;
}

export function UserManualModal({ open, context = "onboarding", onClose, onViewFull }: UserManualModalProps) {
  return (
    <Dialog open={open} onOpenChange={(o) => { if (!o) onClose(); }}>
      <DialogContent className="max-h-[90vh] overflow-y-auto sm:max-w-sm">
        <DialogTitle className="sr-only">Manual do sistema</DialogTitle>
        <UserManual mode="modal" context={context} onClose={onClose} onViewFull={onViewFull} />
      </DialogContent>
    </Dialog>
  );
}
