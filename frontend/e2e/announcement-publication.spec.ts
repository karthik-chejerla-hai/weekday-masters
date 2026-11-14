import { test, expect, json } from './support/fixtures';
test.use({identity:'admin'});
test('uncertain publication retries retain identity and display truthful delivery counts',async({page})=>{
 await page.route('**/api/admin/join-requests',route=>json(route,[]));let requestID='';let calls=0;
 await page.route('**/api/admin/announcements',async route=>{
  const input=route.request().postDataJSON();expect(input.title).toBe('Court closure');expect(input.body).toBe('Friday courts are closed.');
  if(!requestID)requestID=input.request_id;else expect(input.request_id).toBe(requestID);calls++;
  if(calls===1)return json(route,{message:'Could not confirm publication. Retry the same message.'},503);
  await json(route,{id:'11111111-1111-4111-8111-111111111111',request_id:requestID,title:input.title,body:input.body,audience_count:4,queued_count:2,suppressed_count:2,delivery_status:'partially_queued'},201);
 });
 await page.goto('/admin');await page.getByLabel('Title',{exact:true}).fill('Court closure');await page.getByLabel('Message',{exact:true}).fill('Friday courts are closed.');
 await page.getByRole('button',{name:'Publish announcement'}).click();await expect(page.getByRole('alert')).toHaveText('Could not confirm publication. Retry the same message.');
 await page.getByRole('button',{name:'Publish announcement'}).click();await expect(page.getByText('Announcement saved for 4 members. Push queued for 2; 2 will not receive push.')).toBeVisible();expect(calls).toBe(2);
 await expect(page.getByLabel('Title',{exact:true})).toHaveValue('');
});
