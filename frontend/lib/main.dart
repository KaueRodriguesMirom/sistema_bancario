import 'dart:convert';
import 'package:flutter/material.dart';
import 'package:flutter_secure_storage/flutter_secure_storage.dart';
import 'package:http/http.dart' as http;
import 'package:intl/intl.dart';

void main() => runApp(const BankApp());

const apiBase = String.fromEnvironment('API_BASE', defaultValue: 'http://localhost:8080/api');

class BankApp extends StatelessWidget {
  const BankApp({super.key});
  @override
  Widget build(BuildContext context) {
    return MaterialApp(
      debugShowCheckedModeBanner: false,
      title: 'Banco Demo',
      theme: ThemeData(useMaterial3: true, colorSchemeSeed: const Color(0xFF006C4C)),
      home: const AuthPage(),
    );
  }
}

class Api {
  static const storage = FlutterSecureStorage();
  static Future<String?> token() => storage.read(key: 'token');
  static Future<Map<String, dynamic>> post(String path, Map<String, dynamic> body, {bool auth=false}) async {
    final headers = {'Content-Type':'application/json'};
    if (auth) headers['Authorization'] = 'Bearer ${await token()}';
    final r = await http.post(Uri.parse('$apiBase/$path'), headers: headers, body: jsonEncode(body));
    final data = jsonDecode(r.body) as Map<String,dynamic>; 
    if (r.statusCode >= 400) throw data['error'] ?? 'Erro na operação';
    return data;
  }
  static Future<dynamic> get(String path) async {
    final r = await http.get(Uri.parse('$apiBase/$path'), headers: {'Authorization':'Bearer ${await token()}'});
    final data = jsonDecode(r.body);
    if (r.statusCode >= 400) throw data['error'] ?? 'Erro na operação';
    return data;
  }
}

class AuthPage extends StatefulWidget { const AuthPage({super.key}); @override State<AuthPage> createState()=>_AuthPageState(); }
class _AuthPageState extends State<AuthPage> {
  bool login=true, loading=false;
  final name=TextEditingController(), email=TextEditingController(), password=TextEditingController();
  Future<void> submit() async {
    setState(()=>loading=true);
    try {
      final data=await Api.post(login?'login':'register', login?{'email':email.text,'password':password.text}:{'name':name.text,'email':email.text,'password':password.text});
      await Api.storage.write(key:'token',value:data['token']);
      if (!mounted) return;
      Navigator.pushReplacement(context,MaterialPageRoute(builder:(_)=>const HomePage()));
    } catch(e) { if(mounted) ScaffoldMessenger.of(context).showSnackBar(SnackBar(content:Text('$e'))); }
    finally { if(mounted)setState(()=>loading=false); }
  }
  @override
  Widget build(BuildContext context) {
    return Scaffold(body: Center(child: ConstrainedBox(constrained: const BoxConstraints(maxWidth:420),child:Padding(padding:const EdgeInsets.all(24),child:Column(mainAxisSize:MainAxisSize.min,children:[
      const Icon(Icons.account_balance_rounded,size:72), const SizedBox(height:16),
      Text(login?'Entrar':'Criar conta',style:Theme.of(context).textTheme.headlineMedium),const SizedBox(height:24),
      if(!login) TextField(controller:name,decoration:const InputDecoration(labelText:'Nome',border:OutlineInputBorder())),if(!login)const SizedBox(height:12),
      TextField(controller:email,decoration:const InputDecoration(labelText:'Email',border:OutlineInputBorder())),const SizedBox(height:12),
      TextField(controller:password,obscureText:true,decoration:const InputDecoration(labelText:'Senha',border:OutlineInputBorder())),const SizedBox(height:16),
      SizedBox(width:double.infinity,height:48,child:FilledButton(onPressed:loading?null:submit,child:Text(login?'Entrar':'Criar conta'))),
      TextButton(onPressed:()=>setState(()=>login=!login),child:Text(login?'Ainda não tenho conta':'Já tenho conta'))
    ])))));
  }
}

class HomePage extends StatefulWidget { const HomePage({super.key}); @override State<HomePage> createState()=>_HomePageState(); }
class _HomePageState extends State<HomePage> {
  int cents=0; List<dynamic> transactions=[]; bool loading=true;
  @override void initState(){super.initState();reload();}
  Future<void> reload() async {
    setState(()=>loading=true);
    try { final b=await Api.get('balance'); final t=await Api.get('transactions'); setState(()=>cents=b['balance_cents'] as int); setState(()=>transactions=t); }
    catch(e){if(mounted)ScaffoldMessenger.of(context).showSnackBar(SnackBar(content:Text('$e')));} finally {if(mounted)setState(()=>loading=false);} 
  }
  Future<void> operation(String kind) async {
    final controller=TextEditingController();
    final ok=await showDialog<bool>(context:context,builder:(c)=>AlertDialog(title:Text(kind=='deposit'?'Depositar':'Saque rápido'),content:TextField(controller:controller,keyboardType:const TextInputType.numberWithOptions(decimal:true),decoration:const InputDecoration(prefixText:'R\$ ',border:OutlineInputBorder())),actions:[TextButton(onPressed:()=>Navigator.pop(c,false),child:const Text('Cancelar')),FilledButton(onPressed:()=>Navigator.pop(c,true),child:const Text('Confirmar'))]));
    if(ok!=true)return;
    final value=double.tryParse(controller.text.replaceAll(',','.'));
    if(value==null||value<=0)return;
    try { await Api.post(kind,{'amount_cents':(value*100).round()},auth:true); await reload(); }
    catch(e){if(mounted)ScaffoldMessenger.of(context).showSnackBar(SnackBar(content:Text('$e')));}
  }
  Future<void> logout() async {await Api.storage.delete(key:'token');if(mounted)Navigator.pushReplacement(context,MaterialPageRoute(builder:(_)=>const AuthPage()));}
  @override Widget build(BuildContext context){
    final brl=NumberFormat.currency(locale:'pt_BR',symbol:'R\$');
    return Scaffold(appBar:AppBar(title:const Text('Minha conta'),actions:[IconButton(onPressed:reload,icon:const Icon(Icons.refresh)),IconButton(onPressed:logout,icon:const Icon(Icons.logout))]),body:loading?const Center(child:CircularProgressIndicator()):RefreshIndicator(onRefresh:reload,child:ListView(padding:const EdgeInsets.all(16),children:[
      Card(child:Padding(padding:const EdgeInsets.all(24),child:Column(crossAxisAlignment:CrossAxisAlignment.start,children:[const Text('Saldo disponível'),const SizedBox(height:8),Text(brl.format(cents/100),style:Theme.of(context).textTheme.displaySmall),const SizedBox(height:20),Row(children:[Expanded(child:FilledButton.icon(onPressed:()=>operation('deposit'),icon:const Icon(Icons.add),label:const Text('Depositar'))),const SizedBox(width:12),Expanded(child:OutlinedButton.icon(onPressed:()=>operation('withdraw'),icon:const Icon(Icons.remove),label:const Text('Sacar')))])]))),
      const SizedBox(height:16),Text('Histórico',style:Theme.of(context).textTheme.titleLarge),const SizedBox(height:8),
      ...transactions.map((t){final deposit=t['kind']=='deposit';return ListTile(leading:CircleAvatar(child:Icon(deposit?Icons.arrow_downward:Icons.arrow_upward)),title:Text(deposit?'Depósito':'Saque rápido'),subtitle:Text(t['created_at'].toString().replaceAll('T',' ').split('.').first),trailing:Text('${deposit?'+':'-'} ${brl.format((t['amount_cents'] as num)/100)}'));})
    ])));
  }
}
