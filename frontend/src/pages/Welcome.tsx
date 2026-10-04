import { Link } from 'react-router-dom';
import { CheckCircle2, Calendar, Wallet } from 'lucide-react';
import { useAuth } from '../context/useAuth';

export default function Welcome() {
  const { user, isAuthenticated, isApproved, login, loginForInvitation, authError, retryAuth } = useAuth();
  return (
    <main className="min-h-screen bg-slate-50 px-4 py-10 sm:py-16">
      <div className="mx-auto max-w-xl">
        <Link to="/" className="mb-8 inline-flex items-center gap-3 text-lg font-bold text-slate-900"><img src="/badminton.svg" alt="" className="h-10 w-10" />Rally</Link>
        <div className="rounded-2xl border border-slate-200 bg-white p-6 shadow-sm sm:p-9">
          <p className="text-sm font-medium text-primary-700">Your club invitation</p>
          <h1 className="mt-2 text-3xl font-semibold tracking-tight text-slate-950">{isAuthenticated && isApproved ? 'You’re ready to play.' : 'Welcome to Rally'}</h1>
          {isAuthenticated && isApproved ? <>
            <p className="mt-4 break-words text-slate-600">Signed in as <strong>{user?.email}</strong>.</p>
            <p className="mt-3 text-slate-600">Your club account is ready. Your balance, history, and saved RSVPs are available.</p>
            <Link to="/dashboard" className="btn-primary mt-6 inline-flex items-center gap-2"><CheckCircle2 className="h-4 w-4" />Open my club</Link>
          </> : <>
            <p className="mt-4 text-slate-600">Sign in with the Google email address shown in your invitation. No new password is needed.</p>
            {isAuthenticated && !isApproved && <p role="alert" className="mt-4 rounded-lg bg-amber-50 p-3 text-sm text-amber-900">{user?.email} does not have approved club access. Choose the Google account listed in your invitation, or contact your club admin.</p>}
            <button type="button" onClick={loginForInvitation || login} className="btn-primary mt-6">{isAuthenticated ? 'Choose another Google account' : 'Sign in with Google'}</button>
          </>}
          {authError && <div role="alert" className="mt-4 rounded-lg bg-red-50 p-3 text-sm text-red-700"><p>{authError}</p>{retryAuth && <button type="button" onClick={retryAuth} className="mt-2 font-medium underline">Try again</button>}</div>}
          <div className="mt-8 space-y-4 border-t border-slate-100 pt-6 text-sm text-slate-600">
            <p className="flex gap-3"><Wallet className="h-5 w-5 shrink-0 text-primary-700" />Your existing balance and recorded games stay with your account.</p>
            <p className="flex gap-3"><Calendar className="h-5 w-5 shrink-0 text-primary-700" />Already marked IN for a session? You do not need to RSVP again.</p>
          </div>
          {isAuthenticated && isApproved && <button type="button" onClick={loginForInvitation || login} className="mt-6 text-sm text-primary-700 underline">Use a different Google account</button>}
          <p className="mt-6 text-xs leading-5 text-slate-500">If sign-in does not work, check the email address in your invitation. Contact your club admin if you still need help.</p>
        </div>
      </div>
    </main>
  );
}
