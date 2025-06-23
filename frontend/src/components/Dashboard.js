import React, { useState, useEffect, useRef, useCallback } from 'react';
import { useAuth } from '../contexts/AuthContext';
import axios from 'axios';
import {
  Container,
  Box,
  Typography,
  Button,
  Paper,
  Tabs,
  Tab,
  List,
  ListItem,
  ListItemText,
  ListItemIcon,
  ListItemSecondaryAction,
  IconButton,
  TextField,
  Divider,
  Snackbar,
  Alert,
  CircularProgress,
  Card,
  CardContent,
  CardMedia,
  Grid,
  Tooltip
} from '@mui/material';
import {
  TextFields as TextIcon,
  Image as ImageIcon,
  InsertDriveFile as FileIcon,
  ContentCopy as CopyIcon,
  Delete as DeleteIcon,
  Refresh as RefreshIcon,
  Logout as LogoutIcon,
  Add as AddIcon,
  ContentPaste as PasteIcon
} from '@mui/icons-material';

// 剪贴板项目类型
const ITEM_TYPES = {
  TEXT: 'text',
  IMAGE: 'image',
  FILE: 'file'
};

const Dashboard = () => {
  const { currentUser, logout } = useAuth();
  const [activeTab, setActiveTab] = useState(0);
  const [clipboardItems, setClipboardItems] = useState([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState('');
  const [success, setSuccess] = useState('');
  const [textInput, setTextInput] = useState('');
  const [selectedFile, setSelectedFile] = useState(null);
  const fileInputRef = useRef(null);
  const pasteAreaRef = useRef(null);
  
  // 轮询间隔（毫秒）
  const POLLING_INTERVAL = 10000; // 10秒
  
  // 初始加载和轮询设置
  useEffect(() => {
    fetchClipboardItems();
    
    // 设置轮询
    const intervalId = setInterval(() => {
      fetchClipboardItems();
    }, POLLING_INTERVAL);
    
    // 清理函数
    return () => clearInterval(intervalId);
  }, []);
  
  // 粘贴事件监听
  useEffect(() => {
    const handlePaste = (e) => {
      const items = e.clipboardData.items;
      
      for (let i = 0; i < items.length; i++) {
        const item = items[i];
        
        // 处理文本
        if (item.kind === 'string' && item.type.match('^text/plain')) {
          item.getAsString((text) => {
            setTextInput(text);
          });
          return;
        }
        
        // 处理图片
        if (item.kind === 'file' && item.type.match('^image/')) {
          const file = item.getAsFile();
          handleImageUpload(file);
          return;
        }
      }
    };
    
    const pasteArea = pasteAreaRef.current;
    if (pasteArea) {
      pasteArea.addEventListener('paste', handlePaste);
    }
    
    return () => {
      if (pasteArea) {
        pasteArea.removeEventListener('paste', handlePaste);
      }
    };
  }, []);
  
  // 获取剪贴板项目
  const fetchClipboardItems = async () => {
    try {
      setLoading(true);
      const response = await axios.get('/api/clipboard');
      setClipboardItems(response.data);
      setError('');
    } catch (err) {
      setError('获取剪贴板内容失败');
      console.error(err);
    } finally {
      setLoading(false);
    }
  };
  
  // 添加文本
  const handleAddText = async () => {
    if (!textInput.trim()) return;
    
    try {
      await axios.post('/api/clipboard/text', textInput, {
        headers: {
          'Content-Type': 'text/plain'
        }
      });
      setTextInput('');
      setSuccess('文本已添加到剪贴板');
      fetchClipboardItems();
    } catch (err) {
      setError('添加文本失败');
      console.error(err);
    }
  };
  
  // 上传文件
  const handleFileUpload = async (event) => {
    const file = event.target.files[0];
    if (!file) return;
    
    const formData = new FormData();
    formData.append('file', file);
    
    try {
      await axios.post('/api/clipboard/file', formData);
      setSuccess('文件已上传到剪贴板');
      fetchClipboardItems();
    } catch (err) {
      setError('上传文件失败');
      console.error(err);
    }
    
    // 重置文件输入
    if (fileInputRef.current) {
      fileInputRef.current.value = '';
    }
  };
  
  // 上传图片
  const handleImageUpload = async (file) => {
    if (!file) return;
    
    const formData = new FormData();
    formData.append('file', file);
    
    try {
      await axios.post('/api/clipboard/image', formData);
      setSuccess('图片已添加到剪贴板');
      fetchClipboardItems();
    } catch (err) {
      setError('添加图片失败');
      console.error(err);
    }
  };
  
  // 复制文本到剪贴板
  const copyToClipboard = async (text) => {
    try {
      await navigator.clipboard.writeText(text);
      setSuccess('已复制到剪贴板');
    } catch (err) {
      setError('复制失败');
      console.error(err);
    }
  };
  
  // 删除项目
  const handleDelete = async (id) => {
    try {
      await axios.delete(`/api/clipboard/${id}`);
      setSuccess('已删除');
      fetchClipboardItems();
    } catch (err) {
      setError('删除失败');
      console.error(err);
    }
  };
  
  // 处理标签页变化
  const handleTabChange = (event, newValue) => {
    setActiveTab(newValue);
  };
  
  // 处理登出
  const handleLogout = () => {
    logout();
  };
  
  // 关闭提示
  const handleCloseAlert = () => {
    setSuccess('');
    setError('');
  };
  
  // 过滤当前标签页的项目
  const filteredItems = () => {
    if (activeTab === 0) return clipboardItems;
    
    const types = [ITEM_TYPES.TEXT, ITEM_TYPES.IMAGE, ITEM_TYPES.FILE];
    return clipboardItems.filter(item => item.type === types[activeTab - 1]);
  };
  
  // 渲染项目列表
  const renderClipboardItems = () => {
    const items = filteredItems();
    
    if (items.length === 0) {
      return (
        <Box sx={{ textAlign: 'center', py: 4 }}>
          <Typography variant="body1" color="text.secondary">
            暂无内容
          </Typography>
        </Box>
      );
    }
    
    return (
      <Grid container spacing={2}>
        {items.map(item => (
          <Grid item xs={12} key={item.id}>
            <Card variant="outlined">
              <CardContent>
                <Box sx={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', mb: 1 }}>
                  <Box sx={{ display: 'flex', alignItems: 'center' }}>
                    {item.type === ITEM_TYPES.TEXT && <TextIcon color="primary" />}
                    {item.type === ITEM_TYPES.IMAGE && <ImageIcon color="primary" />}
                    {item.type === ITEM_TYPES.FILE && <FileIcon color="primary" />}
                    <Typography variant="body2" color="text.secondary" sx={{ ml: 1 }}>
                      {new Date(item.created_at).toLocaleString()}
                    </Typography>
                  </Box>
                  <Box>
                    {item.type === ITEM_TYPES.TEXT && (
                      <Tooltip title="复制到剪贴板">
                        <IconButton size="small" onClick={() => copyToClipboard(item.content)}>
                          <CopyIcon fontSize="small" />
                        </IconButton>
                      </Tooltip>
                    )}
                    <Tooltip title="删除">
                      <IconButton size="small" onClick={() => handleDelete(item.id)}>
                        <DeleteIcon fontSize="small" />
                      </IconButton>
                    </Tooltip>
                  </Box>
                </Box>
                
                {item.type === ITEM_TYPES.TEXT && (
                  <Typography variant="body1" sx={{ whiteSpace: 'pre-wrap', wordBreak: 'break-word' }}>
                    {item.content}
                  </Typography>
                )}
                
                {item.type === ITEM_TYPES.IMAGE && (
                  <ImageWithAuth itemId={item.id} />
                )}
                
                {item.type === ITEM_TYPES.FILE && (
                  <Box>
                    <Typography variant="body1">
                      {item.filename}
                    </Typography>
                    <Button 
                      variant="outlined" 
                      size="small" 
                      startIcon={<CopyIcon />} 
                      sx={{ mt: 1 }}
                      href={`/api/clipboard/file/${item.id}`}
                      download={item.filename}
                    >
                      下载文件
                    </Button>
                  </Box>
                )}
              </CardContent>
            </Card>
          </Grid>
        ))}
      </Grid>
    );
  };
  
  return (
    <Container maxWidth="md" sx={{ py: 4 }} ref={pasteAreaRef} tabIndex="0">
      <Box sx={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', mb: 4 }}>
        <Typography variant="h4" component="h1">
          WeiCopy
        </Typography>
        <Box>
          <Tooltip title="刷新">
            <IconButton onClick={fetchClipboardItems} disabled={loading}>
              <RefreshIcon />
            </IconButton>
          </Tooltip>
          <Tooltip title="登出">
            <IconButton onClick={handleLogout}>
              <LogoutIcon />
            </IconButton>
          </Tooltip>
        </Box>
      </Box>
      
      <Paper sx={{ mb: 4 }}>
        <Tabs 
          value={activeTab} 
          onChange={handleTabChange} 
          variant="fullWidth"
          indicatorColor="primary"
          textColor="primary"
        >
          <Tab label="全部" />
          <Tab label="文本" />
          <Tab label="图片" />
          <Tab label="文件" />
        </Tabs>
      </Paper>
      
      <Paper sx={{ p: 3, mb: 4 }}>
        <Typography variant="h6" gutterBottom>
          添加到剪贴板
        </Typography>
        <Divider sx={{ mb: 2 }} />
        
        <Box sx={{ mb: 3 }}>
          <Typography variant="subtitle1" gutterBottom>
            文本
          </Typography>
          <Box sx={{ display: 'flex' }}>
            <TextField
              fullWidth
              multiline
              rows={3}
              placeholder="输入或粘贴文本..."
              value={textInput}
              onChange={(e) => setTextInput(e.target.value)}
              variant="outlined"
            />
            <Button
              variant="contained"
              sx={{ ml: 1, minWidth: '120px' }}
              onClick={handleAddText}
              disabled={!textInput.trim()}
              startIcon={<AddIcon />}
            >
              添加文本
            </Button>
          </Box>
        </Box>
        
        <Divider sx={{ my: 2 }} />
        
        <Box sx={{ mb: 3 }}>
          <Typography variant="subtitle1" gutterBottom>
            文件/图片
          </Typography>
          <Box sx={{ display: 'flex', alignItems: 'center' }}>
            <Button
              variant="contained"
              component="label"
              startIcon={<AddIcon />}
            >
              选择文件
              <input
                type="file"
                hidden
                onChange={handleFileUpload}
                ref={fileInputRef}
              />
            </Button>
            <Typography variant="body2" color="text.secondary" sx={{ ml: 2 }}>
              或直接粘贴图片 (Ctrl+V)
            </Typography>
          </Box>
        </Box>
        
        <Divider sx={{ my: 2 }} />
        
        <Box>
          <Typography variant="subtitle1" gutterBottom>
            命令行使用
          </Typography>
          <Typography variant="body2" color="text.secondary" sx={{ mb: 1 }}>
            上传文本:
          </Typography>
          <TextField
            fullWidth
            size="small"
            variant="outlined"
            value={`curl -X POST -H "Content-Type: text/plain" -H "Authorization: Bearer YOUR_TOKEN" -d "要上传的文本" http://your-server/api/clipboard/text`}
            InputProps={{
              readOnly: true,
              endAdornment: (
                <IconButton size="small" onClick={() => copyToClipboard(`curl -X POST -H "Content-Type: text/plain" -H "Authorization: Bearer YOUR_TOKEN" -d "要上传的文本" http://your-server/api/clipboard/text`)}>
                  <CopyIcon fontSize="small" />
                </IconButton>
              ),
            }}
          />
          
          <Typography variant="body2" color="text.secondary" sx={{ mt: 2, mb: 1 }}>
            上传文件:
          </Typography>
          <TextField
            fullWidth
            size="small"
            variant="outlined"
            value={`curl -X POST -H "Authorization: Bearer YOUR_TOKEN" -F "file=@/path/to/your/file" http://your-server/api/clipboard/file`}
            InputProps={{
              readOnly: true,
              endAdornment: (
                <IconButton size="small" onClick={() => copyToClipboard(`curl -X POST -H "Authorization: Bearer YOUR_TOKEN" -F "file=@/path/to/your/file" http://your-server/api/clipboard/file`)}>
                  <CopyIcon fontSize="small" />
                </IconButton>
              ),
            }}
          />
        </Box>
      </Paper>
      
      <Typography variant="h6" gutterBottom>
        剪贴板内容
      </Typography>
      
      {loading ? (
        <Box sx={{ display: 'flex', justifyContent: 'center', py: 4 }}>
          <CircularProgress />
        </Box>
      ) : (
        renderClipboardItems()
      )}
      
      <Snackbar open={!!success} autoHideDuration={3000} onClose={handleCloseAlert}>
        <Alert onClose={handleCloseAlert} severity="success" sx={{ width: '100%' }}>
          {success}
        </Alert>
      </Snackbar>
      
      <Snackbar open={!!error} autoHideDuration={3000} onClose={handleCloseAlert}>
        <Alert onClose={handleCloseAlert} severity="error" sx={{ width: '100%' }}>
          {error}
        </Alert>
      </Snackbar>
    </Container>
  );
};

const ImageWithAuth = ({ itemId }) => {
  const [imageUrl, setImageUrl] = useState('');
  const [loading, setLoading] = useState(true);
  const abortControllerRef = useRef(null);

  // Stable fetch function with cleanup
  const fetchImage = useCallback(async () => {
    abortControllerRef.current?.abort();
    abortControllerRef.current = new AbortController();
    
    try {
      const response = await axios.get(`/api/clipboard/file/${itemId}`, {
        responseType: 'blob',
        signal: abortControllerRef.current.signal
      });
      const url = URL.createObjectURL(response.data);
      setImageUrl(url);
    } catch (err) {
      if (!axios.isCancel(err)) {
        console.error('Failed to load image', err);
      }
    } finally {
      setLoading(false);
    }
  }, [itemId]); // Only recreate when itemId changes

  useEffect(() => {
    fetchImage();
    return () => {
      abortControllerRef.current?.abort();
      if (imageUrl) {
        URL.revokeObjectURL(imageUrl);
      }
    };
  }, [fetchImage]); // Only run when fetchImage changes

  if (loading) return <CircularProgress size={24} />;

  return (
    <Box sx={{ textAlign: 'center' }}>
      <CardMedia
        component="img"
        image={imageUrl}
        alt="剪贴板图片"
        sx={{ maxHeight: 300, width: 'auto', margin: '0 auto' }}
      />
      <Button 
        variant="outlined" 
        size="small" 
        startIcon={<CopyIcon />} 
        sx={{ mt: 1 }}
        onClick={async () => {
          const response = await axios.get(`/api/clipboard/file/${itemId}`, {
            responseType: 'blob'
          });
          const url = URL.createObjectURL(response.data);
          window.open(url, '_blank');
        }}
      >
        查看原图
      </Button>
    </Box>
  );
};

export default Dashboard;